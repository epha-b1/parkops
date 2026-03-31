CREATE TABLE IF NOT EXISTS reservations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id uuid NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES members(id) ON DELETE RESTRICT,
    vehicle_id uuid NOT NULL REFERENCES vehicles(id) ON DELETE RESTRICT,
    status text NOT NULL CHECK (status IN ('hold', 'confirmed', 'cancelled', 'expired')),
    time_window_start timestamptz NOT NULL,
    time_window_end timestamptz NOT NULL,
    stall_count integer NOT NULL CHECK (stall_count > 0),
    rate_plan_id uuid REFERENCES rate_plans(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    confirmed_at timestamptz,
    cancelled_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (time_window_end > time_window_start)
);

CREATE TABLE IF NOT EXISTS capacity_holds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id uuid NOT NULL UNIQUE REFERENCES reservations(id) ON DELETE CASCADE,
    zone_id uuid NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
    stall_count integer NOT NULL CHECK (stall_count > 0),
    expires_at timestamptz NOT NULL,
    released_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS capacity_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id uuid NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
    snapshot_at timestamptz NOT NULL DEFAULT now(),
    authoritative_stalls integer NOT NULL CHECK (authoritative_stalls >= 0)
);

CREATE TABLE IF NOT EXISTS booking_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id uuid NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    event_type text NOT NULL CHECK (event_type IN ('hold_created', 'confirmed', 'cancelled', 'expired', 'hold_released')),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
    detail jsonb
);

CREATE INDEX IF NOT EXISTS idx_reservations_zone_window ON reservations(zone_id, time_window_start, time_window_end);
CREATE INDEX IF NOT EXISTS idx_reservations_status ON reservations(status);
CREATE INDEX IF NOT EXISTS idx_capacity_holds_zone_active ON capacity_holds(zone_id, expires_at) WHERE released_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_capacity_snapshots_zone_time ON capacity_snapshots(zone_id, snapshot_at DESC);
CREATE INDEX IF NOT EXISTS idx_booking_events_reservation_time ON booking_events(reservation_id, occurred_at);
