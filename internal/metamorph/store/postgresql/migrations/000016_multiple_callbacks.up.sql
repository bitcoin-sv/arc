-- Step 1: Add the new 'callback' column
ALTER TABLE global.Transactions
    ADD COLUMN callbacks JSONB;

-- Step 2: Populate the 'callback' column with data from 'callback_url' and 'callback_token'
UPDATE global.Transactions
SET callbacks = json_build_array(
    json_build_object(
        'callback_url', callback_url,
        'callback_token', callback_token
    )
)WHERE LENGTH(callback_url) > 0 OR LENGTH(callback_token) > 0;

-- Step 3: Drop the old 'callback_url' and 'callback_token' columns
ALTER TABLE global.Transactions
DROP COLUMN callback_url,
DROP COLUMN callback_token;
