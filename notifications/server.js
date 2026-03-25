const express = require('express');
const cors = require('cors');
const notificationRoutes = require('./notification-routes');

const app = express();
app.use(cors());
app.use(express.json());
app.use('/api/notifications', notificationRoutes);

const PORT = process.env.NOTIFICATION_PORT || 3001;
app.listen(PORT, () => {
    console.log(`🔔 Notification service running on port ${PORT}`);
});