-- ─── Demo seed data for ParkOps UI ─────────────────────────────────────────
-- Password for all demo users: AdminPass1234
-- (same argon2id hash as admin user)

-- ─── USERS ──────────────────────────────────────────────────────────────────
INSERT INTO users (id, organization_id, username, password_hash, display_name, status, force_password_change) VALUES
    ('dd000001-0000-0000-0000-000000000001', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'operator1', 'argon2id$v=19$m=65536,t=3,p=2$gBb70F3kGCGkdbN2fEkdug$g5F5oED/vT9NffvnJQrmay3FbfMgR9WGqoK9yffnzTE', 'Jane Dispatch', 'active', true),
    ('dd000001-0000-0000-0000-000000000002', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'fleet1', 'argon2id$v=19$m=65536,t=3,p=2$gBb70F3kGCGkdbN2fEkdug$g5F5oED/vT9NffvnJQrmay3FbfMgR9WGqoK9yffnzTE', 'Bob Fleet', 'active', true),
    ('dd000001-0000-0000-0000-000000000003', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'auditor1', 'argon2id$v=19$m=65536,t=3,p=2$gBb70F3kGCGkdbN2fEkdug$g5F5oED/vT9NffvnJQrmay3FbfMgR9WGqoK9yffnzTE', 'Carol Auditor', 'active', true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT 'dd000001-0000-0000-0000-000000000001'::uuid, id FROM roles WHERE name = 'dispatch_operator'
ON CONFLICT DO NOTHING;
INSERT INTO user_roles (user_id, role_id)
SELECT 'dd000001-0000-0000-0000-000000000002'::uuid, id FROM roles WHERE name = 'fleet_manager'
ON CONFLICT DO NOTHING;
INSERT INTO user_roles (user_id, role_id)
SELECT 'dd000001-0000-0000-0000-000000000003'::uuid, id FROM roles WHERE name = 'auditor'
ON CONFLICT DO NOTHING;

-- ─── FACILITIES / LOTS / ZONES ──────────────────────────────────────────────
INSERT INTO facilities (id, name, address) VALUES
    ('f0000001-0000-0000-0000-000000000001', 'Downtown Garage', '100 Main St, Downtown'),
    ('f0000001-0000-0000-0000-000000000002', 'Airport Terminal Parking', '1 Airport Blvd'),
    ('f0000001-0000-0000-0000-000000000003', 'Waterfront Lot', '25 Harbor Dr')
ON CONFLICT (id) DO NOTHING;

INSERT INTO lots (id, facility_id, name) VALUES
    ('10000001-0000-0000-0000-000000000001', 'f0000001-0000-0000-0000-000000000001', 'Level 1'),
    ('10000001-0000-0000-0000-000000000002', 'f0000001-0000-0000-0000-000000000001', 'Level 2'),
    ('10000001-0000-0000-0000-000000000003', 'f0000001-0000-0000-0000-000000000002', 'Short-Term Lot'),
    ('10000001-0000-0000-0000-000000000004', 'f0000001-0000-0000-0000-000000000002', 'Long-Term Lot'),
    ('10000001-0000-0000-0000-000000000005', 'f0000001-0000-0000-0000-000000000003', 'Open Air Lot')
ON CONFLICT (id) DO NOTHING;

INSERT INTO zones (id, lot_id, name, total_stalls, hold_timeout_minutes) VALUES
    ('20000001-0000-0000-0000-000000000001', '10000001-0000-0000-0000-000000000001', 'Zone A - Compact', 80, 10),
    ('20000001-0000-0000-0000-000000000002', '10000001-0000-0000-0000-000000000001', 'Zone B - Standard', 120, 15),
    ('20000001-0000-0000-0000-000000000003', '10000001-0000-0000-0000-000000000002', 'Zone C - Premium', 40, 20),
    ('20000001-0000-0000-0000-000000000004', '10000001-0000-0000-0000-000000000003', 'Zone D - Hourly', 200, 10),
    ('20000001-0000-0000-0000-000000000005', '10000001-0000-0000-0000-000000000004', 'Zone E - Daily', 500, 30),
    ('20000001-0000-0000-0000-000000000006', '10000001-0000-0000-0000-000000000005', 'Zone F - Waterfront', 60, 15)
ON CONFLICT (id) DO NOTHING;

-- ─── RATE PLANS ─────────────────────────────────────────────────────────────
INSERT INTO rate_plans (id, zone_id, name, rate_cents, period) VALUES
    ('30000001-0000-0000-0000-000000000001', '20000001-0000-0000-0000-000000000001', 'Compact Hourly', 250, 'hourly'),
    ('30000001-0000-0000-0000-000000000002', '20000001-0000-0000-0000-000000000002', 'Standard Daily', 2000, 'daily'),
    ('30000001-0000-0000-0000-000000000003', '20000001-0000-0000-0000-000000000003', 'Premium Monthly', 30000, 'monthly'),
    ('30000001-0000-0000-0000-000000000004', '20000001-0000-0000-0000-000000000004', 'Airport Hourly', 500, 'hourly'),
    ('30000001-0000-0000-0000-000000000005', '20000001-0000-0000-0000-000000000005', 'Airport Daily', 3500, 'daily'),
    ('30000001-0000-0000-0000-000000000006', '20000001-0000-0000-0000-000000000006', 'Waterfront Hourly', 300, 'hourly')
ON CONFLICT (id) DO NOTHING;

-- ─── MEMBERS ────────────────────────────────────────────────────────────────
INSERT INTO members (id, organization_id, display_name, arrears_balance_cents) VALUES
    ('40000001-0000-0000-0000-000000000001', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'Alice Parker', 0),
    ('40000001-0000-0000-0000-000000000002', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'David Chen', 5500),
    ('40000001-0000-0000-0000-000000000003', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'Maria Santos', 0),
    ('40000001-0000-0000-0000-000000000004', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'James Wilson', 12000),
    ('40000001-0000-0000-0000-000000000005', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'Sarah Lee', 0),
    ('40000001-0000-0000-0000-000000000006', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'Tom Rivera', 25000)
ON CONFLICT (id) DO NOTHING;

-- ─── VEHICLES ───────────────────────────────────────────────────────────────
INSERT INTO vehicles (id, organization_id, plate_number, make, model) VALUES
    ('50000001-0000-0000-0000-000000000001', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'ABC-1234', 'Toyota', 'Camry'),
    ('50000001-0000-0000-0000-000000000002', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'DEF-5678', 'Honda', 'Civic'),
    ('50000001-0000-0000-0000-000000000003', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'GHI-9012', 'Ford', 'F-150'),
    ('50000001-0000-0000-0000-000000000004', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'JKL-3456', 'Tesla', 'Model 3'),
    ('50000001-0000-0000-0000-000000000005', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'MNO-7890', 'BMW', 'X5')
ON CONFLICT (id) DO NOTHING;

-- ─── DRIVERS ────────────────────────────────────────────────────────────────
INSERT INTO drivers (id, organization_id, member_id, licence_number) VALUES
    ('60000001-0000-0000-0000-000000000001', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '40000001-0000-0000-0000-000000000001', 'DL-001-2025'),
    ('60000001-0000-0000-0000-000000000002', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '40000001-0000-0000-0000-000000000002', 'DL-002-2025'),
    ('60000001-0000-0000-0000-000000000003', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', '40000001-0000-0000-0000-000000000004', 'DL-003-2025')
ON CONFLICT (id) DO NOTHING;

-- ─── DEVICES ────────────────────────────────────────────────────────────────
INSERT INTO devices (id, organization_id, device_key, device_type, zone_id, status) VALUES
    ('70000001-0000-0000-0000-000000000001', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'cam-entrance-a1', 'camera', '20000001-0000-0000-0000-000000000001', 'online'),
    ('70000001-0000-0000-0000-000000000002', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'gate-exit-b1', 'gate', '20000001-0000-0000-0000-000000000002', 'online'),
    ('70000001-0000-0000-0000-000000000003', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'sensor-c1', 'geomagnetic', '20000001-0000-0000-0000-000000000003', 'offline')
ON CONFLICT (id) DO NOTHING;

-- ─── RESERVATIONS ───────────────────────────────────────────────────────────
INSERT INTO reservations (id, zone_id, member_id, vehicle_id, status, time_window_start, time_window_end, stall_count, rate_plan_id, confirmed_at) VALUES
    ('80000001-0000-0000-0000-000000000001', '20000001-0000-0000-0000-000000000001', '40000001-0000-0000-0000-000000000001', '50000001-0000-0000-0000-000000000001', 'confirmed', now() - interval '2 hours', now() + interval '6 hours', 1, '30000001-0000-0000-0000-000000000001', now() - interval '2 hours'),
    ('80000001-0000-0000-0000-000000000002', '20000001-0000-0000-0000-000000000002', '40000001-0000-0000-0000-000000000002', '50000001-0000-0000-0000-000000000002', 'confirmed', now() - interval '1 hour', now() + interval '8 hours', 2, '30000001-0000-0000-0000-000000000002', now() - interval '1 hour'),
    ('80000001-0000-0000-0000-000000000003', '20000001-0000-0000-0000-000000000004', '40000001-0000-0000-0000-000000000004', '50000001-0000-0000-0000-000000000004', 'confirmed', now(), now() + interval '4 hours', 1, '30000001-0000-0000-0000-000000000004', now()),
    ('80000001-0000-0000-0000-000000000004', '20000001-0000-0000-0000-000000000003', '40000001-0000-0000-0000-000000000003', '50000001-0000-0000-0000-000000000003', 'hold', now(), now() + interval '3 hours', 1, '30000001-0000-0000-0000-000000000003', NULL),
    ('80000001-0000-0000-0000-000000000005', '20000001-0000-0000-0000-000000000006', '40000001-0000-0000-0000-000000000005', '50000001-0000-0000-0000-000000000005', 'confirmed', now() - interval '30 minutes', now() + interval '2 hours', 3, '30000001-0000-0000-0000-000000000006', now() - interval '30 minutes'),
    ('80000001-0000-0000-0000-000000000006', '20000001-0000-0000-0000-000000000001', '40000001-0000-0000-0000-000000000006', '50000001-0000-0000-0000-000000000003', 'cancelled', now() - interval '5 hours', now() - interval '1 hour', 1, '30000001-0000-0000-0000-000000000001', NULL)
ON CONFLICT (id) DO NOTHING;

-- Capacity holds for the hold reservation
INSERT INTO capacity_holds (id, zone_id, reservation_id, stall_count, expires_at) VALUES
    ('90000001-0000-0000-0000-000000000001', '20000001-0000-0000-0000-000000000003', '80000001-0000-0000-0000-000000000004', 1, now() + interval '20 minutes')
ON CONFLICT (id) DO NOTHING;

-- ─── BOOKING EVENTS ─────────────────────────────────────────────────────────
INSERT INTO booking_events (reservation_id, event_type, occurred_at, detail) VALUES
    ('80000001-0000-0000-0000-000000000001', 'hold_created', now() - interval '2 hours 5 minutes', '{"stall_count":1}'),
    ('80000001-0000-0000-0000-000000000001', 'confirmed', now() - interval '2 hours', '{}'),
    ('80000001-0000-0000-0000-000000000002', 'hold_created', now() - interval '1 hour 10 minutes', '{"stall_count":2}'),
    ('80000001-0000-0000-0000-000000000002', 'confirmed', now() - interval '1 hour', '{}'),
    ('80000001-0000-0000-0000-000000000003', 'hold_created', now() - interval '5 minutes', '{"stall_count":1}'),
    ('80000001-0000-0000-0000-000000000003', 'confirmed', now(), '{}'),
    ('80000001-0000-0000-0000-000000000004', 'hold_created', now(), '{"stall_count":1}'),
    ('80000001-0000-0000-0000-000000000006', 'hold_created', now() - interval '5 hours 5 minutes', '{"stall_count":1}'),
    ('80000001-0000-0000-0000-000000000006', 'cancelled', now() - interval '4 hours', '{}')
ON CONFLICT DO NOTHING;

-- ─── CAPACITY SNAPSHOTS ─────────────────────────────────────────────────────
INSERT INTO capacity_snapshots (zone_id, snapshot_at, authoritative_stalls) VALUES
    ('20000001-0000-0000-0000-000000000001', now() - interval '2 hours', 79),
    ('20000001-0000-0000-0000-000000000001', now() - interval '1 hour', 78),
    ('20000001-0000-0000-0000-000000000001', now(), 79),
    ('20000001-0000-0000-0000-000000000002', now() - interval '1 hour', 118),
    ('20000001-0000-0000-0000-000000000002', now(), 118),
    ('20000001-0000-0000-0000-000000000003', now(), 39),
    ('20000001-0000-0000-0000-000000000004', now() - interval '30 minutes', 199),
    ('20000001-0000-0000-0000-000000000004', now(), 199),
    ('20000001-0000-0000-0000-000000000005', now(), 500),
    ('20000001-0000-0000-0000-000000000006', now(), 57)
ON CONFLICT DO NOTHING;

-- ─── DEVICE EVENTS ──────────────────────────────────────────────────────────
INSERT INTO device_events (device_id, event_key, sequence_number, event_type, payload, received_at, processed) VALUES
    ('70000001-0000-0000-0000-000000000001', 'evt-cam-001', 1, 'vehicle_enter', '{"plate":"ABC-1234"}', now() - interval '2 hours', true),
    ('70000001-0000-0000-0000-000000000001', 'evt-cam-002', 2, 'vehicle_enter', '{"plate":"DEF-5678"}', now() - interval '1 hour', true),
    ('70000001-0000-0000-0000-000000000002', 'evt-gate-001', 1, 'gate_open', '{"direction":"exit"}', now() - interval '30 minutes', true),
    ('70000001-0000-0000-0000-000000000003', 'evt-sensor-001', 1, 'occupancy_change', '{"stall":"C1-05","occupied":true}', now() - interval '15 minutes', true)
ON CONFLICT (event_key) DO NOTHING;

-- ─── EXCEPTIONS ─────────────────────────────────────────────────────────────
INSERT INTO exceptions (id, device_id, exception_type, status, created_at) VALUES
    ('e0000001-0000-0000-0000-000000000001', '70000001-0000-0000-0000-000000000003', 'sensor_offline', 'open', now() - interval '10 minutes'),
    ('e0000001-0000-0000-0000-000000000002', '70000001-0000-0000-0000-000000000002', 'gate_stuck', 'open', now() - interval '5 minutes'),
    ('e0000001-0000-0000-0000-000000000003', '70000001-0000-0000-0000-000000000001', 'camera_error', 'acknowledged', now() - interval '2 hours')
ON CONFLICT (id) DO NOTHING;

-- ─── NOTIFICATION TOPICS ────────────────────────────────────────────────────
INSERT INTO notification_topics (name) VALUES
    ('booking_success'), ('booking_changed'), ('expiry_approaching'), ('arrears_reminder'), ('task_reminder')
ON CONFLICT (name) DO NOTHING;

-- Subscribe admin to all topics
INSERT INTO notification_subscriptions (user_id, topic_id)
SELECT '11111111-1111-1111-1111-111111111111'::uuid, id FROM notification_topics
ON CONFLICT DO NOTHING;

-- ─── NOTIFICATIONS ──────────────────────────────────────────────────────────
INSERT INTO notifications (id, user_id, topic_id, title, body, read, dismissed, created_at)
SELECT
    gen_random_uuid(),
    '11111111-1111-1111-1111-111111111111'::uuid,
    nt.id,
    'Booking confirmed for Alice Parker',
    'Reservation 80000001 has been confirmed in Zone A - Compact.',
    false,
    false,
    now() - interval '1 hour'
FROM notification_topics nt WHERE nt.name = 'booking_success';

INSERT INTO notifications (id, user_id, topic_id, title, body, read, dismissed, created_at)
SELECT
    gen_random_uuid(),
    '11111111-1111-1111-1111-111111111111'::uuid,
    nt.id,
    'Hold expiring soon',
    'Reservation 80000001-4 in Zone C - Premium expires in 20 minutes.',
    false,
    false,
    now() - interval '5 minutes'
FROM notification_topics nt WHERE nt.name = 'expiry_approaching';

INSERT INTO notifications (id, user_id, topic_id, title, body, read, dismissed, created_at)
SELECT
    gen_random_uuid(),
    '11111111-1111-1111-1111-111111111111'::uuid,
    nt.id,
    'Arrears reminder: Tom Rivera',
    'Member Tom Rivera has an outstanding balance of $250.00.',
    true,
    false,
    now() - interval '3 hours'
FROM notification_topics nt WHERE nt.name = 'arrears_reminder';

-- ─── CAMPAIGNS AND TASKS ───────────────────────────────────────────────────
INSERT INTO campaigns (id, title, description, target_role, created_by) VALUES
    ('c0000001-0000-0000-0000-000000000001', 'Morning Safety Checks', 'Daily pre-open safety inspection checklist', 'dispatch_operator', '11111111-1111-1111-1111-111111111111'),
    ('c0000001-0000-0000-0000-000000000002', 'Monthly Rate Plan Review', 'Review and adjust rate plans based on occupancy trends', 'facility_admin', '11111111-1111-1111-1111-111111111111')
ON CONFLICT (id) DO NOTHING;

INSERT INTO tasks (id, campaign_id, description, deadline, reminder_interval_minutes, created_by) VALUES
    ('d0000001-0000-0000-0000-000000000001', 'c0000001-0000-0000-0000-000000000001', 'Check all gate mechanisms', now() + interval '2 hours', 30, '11111111-1111-1111-1111-111111111111'),
    ('d0000001-0000-0000-0000-000000000002', 'c0000001-0000-0000-0000-000000000001', 'Verify camera feeds are live', now() + interval '2 hours', 30, '11111111-1111-1111-1111-111111111111'),
    ('d0000001-0000-0000-0000-000000000003', 'c0000001-0000-0000-0000-000000000001', 'Inspect emergency exits', now() + interval '3 hours', 60, '11111111-1111-1111-1111-111111111111'),
    ('d0000001-0000-0000-0000-000000000004', 'c0000001-0000-0000-0000-000000000002', 'Pull occupancy reports for last 30 days', now() + interval '7 days', 1440, '11111111-1111-1111-1111-111111111111'),
    ('d0000001-0000-0000-0000-000000000005', 'c0000001-0000-0000-0000-000000000002', 'Compare competitor pricing', now() + interval '7 days', 1440, '11111111-1111-1111-1111-111111111111')
ON CONFLICT (id) DO NOTHING;

-- ─── TAGS ───────────────────────────────────────────────────────────────────
INSERT INTO tags (id, name) VALUES
    ('a0000001-0000-0000-0000-000000000001', 'vip'),
    ('a0000001-0000-0000-0000-000000000002', 'downtown_monthly'),
    ('a0000001-0000-0000-0000-000000000003', 'airport_frequent'),
    ('a0000001-0000-0000-0000-000000000004', 'arrears_warning')
ON CONFLICT (id) DO NOTHING;

INSERT INTO member_tags (member_id, tag_id, assigned_by) VALUES
    ('40000001-0000-0000-0000-000000000001', 'a0000001-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111'),
    ('40000001-0000-0000-0000-000000000001', 'a0000001-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111'),
    ('40000001-0000-0000-0000-000000000003', 'a0000001-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111'),
    ('40000001-0000-0000-0000-000000000004', 'a0000001-0000-0000-0000-000000000003', '11111111-1111-1111-1111-111111111111'),
    ('40000001-0000-0000-0000-000000000004', 'a0000001-0000-0000-0000-000000000004', '11111111-1111-1111-1111-111111111111'),
    ('40000001-0000-0000-0000-000000000006', 'a0000001-0000-0000-0000-000000000004', '11111111-1111-1111-1111-111111111111')
ON CONFLICT DO NOTHING;

-- ─── SEGMENTS ───────────────────────────────────────────────────────────────
INSERT INTO segment_definitions (id, name, filter_expression, schedule, created_by) VALUES
    ('b0000001-0000-0000-0000-000000000001', 'High Arrears Members', '{"arrears_balance_cents": {"gt": 5000}}', 'nightly', '11111111-1111-1111-1111-111111111111'),
    ('b0000001-0000-0000-0000-000000000002', 'VIP Members', '{"tag": "vip"}', 'manual', '11111111-1111-1111-1111-111111111111'),
    ('b0000001-0000-0000-0000-000000000003', 'Downtown Monthly + Arrears', '{"and": [{"tag": "downtown_monthly"}, {"arrears_balance_cents": {"gt": 0}}]}', 'nightly', '11111111-1111-1111-1111-111111111111')
ON CONFLICT (id) DO NOTHING;

INSERT INTO segment_runs (segment_id, ran_at, member_count, triggered_by) VALUES
    ('b0000001-0000-0000-0000-000000000001', now() - interval '1 day', 3, 'scheduler'),
    ('b0000001-0000-0000-0000-000000000001', now(), 3, 'manual'),
    ('b0000001-0000-0000-0000-000000000002', now(), 1, 'manual')
ON CONFLICT DO NOTHING;

-- ─── AUDIT LOG ENTRIES ──────────────────────────────────────────────────────
INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, detail) VALUES
    ('11111111-1111-1111-1111-111111111111', 'login', 'user', '11111111-1111-1111-1111-111111111111', '{"ip":"127.0.0.1"}'),
    ('11111111-1111-1111-1111-111111111111', 'facility_create', 'facility', 'f0000001-0000-0000-0000-000000000001', '{"name":"Downtown Garage"}'),
    ('11111111-1111-1111-1111-111111111111', 'reservation_confirm', 'reservation', '80000001-0000-0000-0000-000000000001', '{"member":"Alice Parker","zone":"Zone A"}'),
    ('11111111-1111-1111-1111-111111111111', 'tag_create', 'tag', 'a0000001-0000-0000-0000-000000000001', '{"name":"vip"}'),
    ('11111111-1111-1111-1111-111111111111', 'segment_run', 'segment', 'b0000001-0000-0000-0000-000000000001', '{"member_count":3}'),
    ('dd000001-0000-0000-0000-000000000001', 'exception_acknowledge', 'exception', 'e0000001-0000-0000-0000-000000000003', '{"type":"camera_error"}'),
    ('11111111-1111-1111-1111-111111111111', 'member_balance_adjust', 'member', '40000001-0000-0000-0000-000000000006', '{"amount_cents":25000,"reason":"overdue invoice"}'),
    ('11111111-1111-1111-1111-111111111111', 'campaign_create', 'campaign', 'c0000001-0000-0000-0000-000000000001', '{"title":"Morning Safety Checks"}')
ON CONFLICT DO NOTHING;

-- ─── EXPORTS ────────────────────────────────────────────────────────────────
INSERT INTO exports (id, requested_by, format, scope, status, file_path, completed_at) VALUES
    ('ee000001-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'csv', 'bookings', 'ready', 'id,zone_id,status,stall_count,created_at', now() - interval '30 minutes'),
    ('ee000001-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111', 'csv', 'occupancy', 'ready', 'snapshot_at,zone_id,authoritative_stalls', now() - interval '15 minutes')
ON CONFLICT (id) DO NOTHING;
