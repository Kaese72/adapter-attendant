-- What the last sync deployed, as handed to ArgoCD by the plugin endpoint.
-- Both stay NULL for adapters that have never been synced since the move to
-- ArgoCD, which keeps them out of the plugin's answer until they are.
ALTER TABLE adapters ADD COLUMN syncedImage VARCHAR(320) NULL;
-- The adapter's whole environment as a JSON object: its arguments as
-- ADAPTER_*, plus the HUEMIE_* values (including the enroll token minted at
-- sync time). A snapshot, so that edits only take effect on the next sync.
ALTER TABLE adapters ADD COLUMN syncedEnvironment LONGTEXT NULL;
-- Bumped on every sync and put on the adapter's pod template, so that a sync
-- always restarts the adapter: its environment is read only at startup.
ALTER TABLE adapters ADD COLUMN syncGeneration BIGINT UNSIGNED NOT NULL DEFAULT 0;
