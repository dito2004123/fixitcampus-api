const express = require('express');
const { MongoClient } = require('mongodb');

const app = express();
app.use(express.json());

// Konfigurasi MongoDB dari environment variables
const mongoUrl = `mongodb://${process.env.DB_USERNAME}:${process.env.DB_PASSWORD}@${process.env.DB_HOST}:${process.env.DB_PORT}`;
const client = new MongoClient(mongoUrl);
let notificationsCollection;

async function connectDb() {
    try {
        await client.connect();
        console.log('Connected to MongoDB');
        const database = client.db(process.env.DB_DATABASE);
        notificationsCollection = database.collection('notifications');
        // Buat index untuk TTL (Time-To-Live), notifikasi akan otomatis terhapus setelah 1 jam
        await notificationsCollection.createIndex({ "createdAt": 1 }, { expireAfterSeconds: 3600 });
    } catch (err) {
        console.error('Failed to connect to MongoDB', err);
        process.exit(1);
    }
}

// Endpoint: POST /notifications
app.post('/notifications', async (req, res) => {
    if (!notificationsCollection) {
        return res.status(503).json({ error: 'Service not ready, DB not connected' });
    }
    
    const { event, message } = req.body;
    if (!event || !message) {
        return res.status(400).json({ error: 'Event and message are required' });
    }

    const newNotification = {
        event,
        message,
        createdAt: new Date(),
    };

    try {
        const result = await notificationsCollection.insertOne(newNotification);
        console.log(`Notification logged: ${message}`);
        res.status(201).json(result.ops[0]);
    } catch (err) {
        console.error('Failed to log notification', err);
        res.status(500).json({ error: 'Could not save notification' });
    }
});

// Endpoint: GET /notifications
app.get('/notifications', async (req, res) => {
    if (!notificationsCollection) {
        return res.status(503).json({ error: 'Service not ready, DB not connected' });
    }

    try {
        const notifications = await notificationsCollection.find().sort({createdAt: -1}).limit(20).toArray();
        res.status(200).json(notifications);
    } catch (err) {
        console.error('Failed to get notifications', err);
        res.status(500).json({ error: 'Could not retrieve notifications' });
    }
});


const PORT = 8083;
app.listen(PORT, () => {
    console.log(`Notification service running on port ${PORT}`);
    connectDb();
});
