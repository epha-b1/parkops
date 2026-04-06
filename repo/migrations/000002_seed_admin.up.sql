INSERT INTO users (
    id,
    username,
    password_hash,
    display_name,
    status,
    failed_login_count,
    force_password_change,
    created_at,
    updated_at
) VALUES (
    '11111111-1111-1111-1111-111111111111',
    'admin',
    'argon2id$v=19$m=65536,t=3,p=2$gBb70F3kGCGkdbN2fEkdug$g5F5oED/vT9NffvnJQrmay3FbfMgR9WGqoK9yffnzTE',
    'System Admin',
    'active',
    0,
    true,
    now(),
    now()
)
ON CONFLICT (username) DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT
    '11111111-1111-1111-1111-111111111111'::uuid,
    r.id
FROM roles r
WHERE r.name = 'facility_admin'
ON CONFLICT DO NOTHING;
