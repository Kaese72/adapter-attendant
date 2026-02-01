CREATE TRIGGER adapterConfiguration_touch_adapter
AFTER UPDATE ON adapterConfiguration
FOR EACH ROW
UPDATE adapters
SET updated = CURRENT_TIMESTAMP
WHERE id = NEW.adapterId;
