import os
from flask import Flask, jsonify, request
import redis

app = Flask(__name__)

# Konfigurasi Redis dari environment variables
redis_host = os.environ.get('REDIS_HOST', 'localhost')
redis_port = int(os.environ.get('REDIS_PORT', 6379))
# Inisialisasi koneksi Redis
try:
    r = redis.Redis(host=redis_host, port=redis_port, db=0, decode_responses=True)
    r.ping()
    print("Connected to Redis")
except redis.exceptions.ConnectionError as e:
    print(f"Could not connect to Redis: {e}")
    r = None

# Inisialisasi counter jika belum ada
if r:
    r.setnx('tickets:open', 0)
    r.setnx('tickets:closed', 0)

@app.route('/reports/summary', methods=['GET'])
def get_summary():
    """Endpoint publik untuk mendapatkan ringkasan laporan."""
    if not r:
        return jsonify({"error": "Redis service not available"}), 503
        
    open_tickets = r.get('tickets:open') or 0
    closed_tickets = r.get('tickets:closed') or 0
    
    summary = {
        "open_tickets": int(open_tickets),
        "closed_tickets": int(closed_tickets),
        "total_tickets": int(open_tickets) + int(closed_tickets)
    }
    return jsonify(summary)

@app.route('/reports/update', methods=['POST'])
def update_report():
    """Endpoint internal untuk memperbarui data laporan."""
    if not r:
        return jsonify({"error": "Redis service not available"}), 503

    data = request.get_json()
    event_type = data.get('event') # Misal: "ticket_created", "ticket_closed"

    if event_type == 'ticket_created':
        new_total = r.incr('tickets:open')
        return jsonify({"status": "success", "open_tickets": new_total})
    elif event_type == 'ticket_closed':
        r.decr('tickets:open')
        new_total = r.incr('tickets:closed')
        return jsonify({"status": "success", "closed_tickets": new_total})
    else:
        return jsonify({"error": "Invalid event type"}), 400

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
