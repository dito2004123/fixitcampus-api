package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
)

var db *sql.DB
// Ambil kunci rahasia dari environment variable
var jwtKey = []byte(os.Getenv("JWT_SECRET_KEY"))

type Ticket struct {
	ID          int    `json:"id"`
	UserID      int    `json:"user_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// Claims custom untuk JWT
type Claims struct {
	Data struct {
		UserID int    `json:"user_id"`
		Role   string `json:"role"`
	} `json:"data"`
	jwt.RegisteredClaims
}

// --- Middleware untuk otorisasi ---
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error": "Authorization header required"}`, http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, `{"error": "Invalid or expired token"}`, http.StatusUnauthorized)
			return
		}
		
		// Tambahkan info user ke header untuk digunakan oleh handler selanjutnya
		r.Header.Set("X-User-ID", fmt.Sprintf("%d", claims.Data.UserID))
		r.Header.Set("X-User-Role", claims.Data.Role)
		
		next.ServeHTTP(w, r)
	}
}


func main() {
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USERNAME"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_DATABASE"))

	var err error
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()
	createTable()

	http.HandleFunc("/tickets/", handleTickets)
	log.Println("Ticket service running on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}

func handleTickets(w http.ResponseWriter, r *http.Request) {
	// Middleware otorisasi dipanggil di sini, di awal handler utama
	authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		pathParts := strings.Split(r.URL.Path, "/")
		userRole := r.Header.Get("X-User-Role")

		switch r.Method {
		case "GET":
			if len(pathParts) >= 3 && pathParts[2] != "" {
				id, _ := strconv.Atoi(pathParts[2])
				getTicket(w, r, id)
			} else {
				if userRole != "admin" {
					http.Error(w, `{"error": "Forbidden: Admin access required"}`, http.StatusForbidden)
					return
				}
				getAllTickets(w, r)
			}
		case "POST":
			createTicket(w, r)
		case "PUT":
			if len(pathParts) < 3 || pathParts[2] == "" {
				http.Error(w, `{"error": "Ticket ID is required"}`, http.StatusBadRequest)
				return
			}
			if userRole != "admin" {
				http.Error(w, `{"error": "Forbidden: Admin access required"}`, http.StatusForbidden)
				return
			}
			id, _ := strconv.Atoi(pathParts[2])
			updateTicketStatus(w, r, id)
		default:
			http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}).ServeHTTP(w, r)
}

// ... (fungsi createTable, getAllTickets, getTicket, updateTicketStatus tidak berubah signifikan, KECUALI createTicket)

func createTicket(w http.ResponseWriter, r *http.Request) {
	var t Ticket
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}
	// Ambil User ID dari token yang sudah divalidasi, bukan dari body request
	userID, _ := strconv.Atoi(r.Header.Get("X-User-ID"))
	t.UserID = userID

	err := db.QueryRow(
		"INSERT INTO tickets (user_id, title, description) VALUES ($1, $2, $3) RETURNING id, status",
		t.UserID, t.Title, t.Description,
	).Scan(&t.ID, &t.Status)

	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to create ticket: %v"}`, err), http.StatusInternalServerError)
		return
	}
	
	go func() {
		jsonData, _ := json.Marshal(map[string]string{"event": "ticket_created"})
		http.Post("http://reporting-service:5000/reports/update", "application/json", bytes.NewBuffer(jsonData))
	}()

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

// --- Sisanya (createTable, getAllTickets, dll) bisa dimasukkan di sini, sama seperti sebelumnya ---
func createTable() {
	query := `CREATE TABLE IF NOT EXISTS tickets (
        id SERIAL PRIMARY KEY, user_id INT NOT NULL, title VARCHAR(255) NOT NULL,
        description TEXT, status VARCHAR(50) DEFAULT 'open', created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`
	if _, err := db.Exec(query); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
}

func getAllTickets(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, user_id, title, description, status FROM tickets ORDER BY id DESC")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Query error: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	tickets := []Ticket{}
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Scan error: %v"}`, err), http.StatusInternalServerError)
			return
		}
		tickets = append(tickets, t)
	}
	json.NewEncoder(w).Encode(tickets)
}

func getTicket(w http.ResponseWriter, r *http.Request, id int) {
	var t Ticket
	err := db.QueryRow("SELECT id, user_id, title, description, status FROM tickets WHERE id = $1", id).Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error": "Ticket not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf(`{"error": "Query error: %v"}`, err), http.StatusInternalServerError)
		}
		return
	}
	json.NewEncoder(w).Encode(t)
}

func updateTicketStatus(w http.ResponseWriter, r *http.Request, id int) {
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}
	validStatuses := []string{"open", "in_progress", "closed"}
	isValidStatus := false
	for _, s := range validStatuses {
		if s == payload.Status {
			isValidStatus = true
			break
		}
	}
	if !isValidStatus {
		http.Error(w, `{"error": "Invalid status. Must be one of: open, in_progress, closed"}`, http.StatusBadRequest)
		return
	}
	res, err := db.Exec("UPDATE tickets SET status = $1 WHERE id = $2", payload.Status, id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to update ticket: %v"}`, err), http.StatusInternalServerError)
		return
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, `{"error": "Ticket not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "Ticket status updated successfully"})
}