-- Development seed examples.
-- Current MVP still recommends registering users through the auth API so that
-- password hashing stays consistent with the application code.

-- Example: grant stream entitlement to a user after the track has been imported.
-- Replace the email/title values before running.
INSERT INTO user_entitlements (user_id, track_id, access_type)
SELECT u.id, t.id, 'STREAM'
FROM users u
JOIN tracks t ON t.title = 'Song A'
WHERE u.email = 'demo@example.com'
ON CONFLICT (user_id, track_id) DO NOTHING;
