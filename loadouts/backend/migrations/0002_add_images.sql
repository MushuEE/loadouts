-- Add image support to items and user metadata

ALTER TABLE items ADD COLUMN image_url TEXT;

ALTER TABLE user_metadata ADD COLUMN custom_image_url TEXT;
