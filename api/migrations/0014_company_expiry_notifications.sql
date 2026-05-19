ALTER TABLE companies
      ADD COLUMN expiry_notifications_enabled boolean NOT NULL DEFAULT false,
      ADD COLUMN expiry_notification_email text;
