-- Step 1: Add the old 'callback_url' and 'callback_token' columns back
ALTER TABLE global.Transactions
    ADD COLUMN callback_url TEXT,
    ADD COLUMN callback_token TEXT;

-- Step 2: Populate 'callback_url' and 'callback_token' with the first object in the 'callbacks' JSON array
UPDATE global.Transactions
SET callback_url = (callbacks->0->>'callback_url'),
    callback_token = (callbacks->0->>'callback_token')
WHERE jsonb_array_length(callbacks) > 0;

-- Step 3: Drop the new 'callbacks' column
ALTER TABLE global.Transactions
    DROP COLUMN callbacks;
