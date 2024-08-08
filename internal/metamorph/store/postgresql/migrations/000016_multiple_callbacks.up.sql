-- Step 1: Add the new 'callback' column
ALTER TABLE metamorph.transactions
    ADD COLUMN callbacks JSONB;

-- Step 2: Populate the 'callback' column with data from 'callback_url' and 'callback_token'
UPDATE metamorph.transactions
SET callbacks = json_build_array(
    json_build_object(
        'callback_url', callback_url,
        'callback_token', callback_token
    )
)WHERE callback_url != '' OR callback_token != '';

-- Step 3: Drop the old 'callback_url' and 'callback_token' columns
ALTER TABLE metamorph.transactions
DROP COLUMN callback_url,
DROP COLUMN callback_token;