openapi: "3.0.3"
info:
  title: ParkOps API
  version: "1.0.0"
  description: |
    ParkOps Command & Reservation Platform API.
    Offline-first parking operations. All endpoints require session cookie authentication
    unless marked as public. Role-based access is enforced on every route.
servers:
  - url: http://localhost:8080/api
    description: Local development

components:
  securitySchemes:
    sessionCookie:
      type: apiKey
      in: cookie
      name: session_id

  schemas:
    Error:
      type: object
      properties:
        code:
          type: string
          example: VALIDATION_ERROR
        message:
          type: string
          example: Password must be at least 12 characters

    User:
      type: object
      properties:
        id:
          type: string
          format: uuid
        username:
          type: string
        display_name:
          type: string
        status:
          type: string
          enum: [active, inactive, locked]
        roles:
          type: array
          items:
            type: string
        force_password_change:
          type: boolean
        created_at:
          type: string
          format: date-time

    Session:
      type: object
      properties:
        id:
          type: string
          format: uuid
        created_at:
          type: string
          format: date-time
        last_active_at:
          type: string
          format: date-time
        expires_at:
          type: string
          format: date-time

    Facility:
      type: object
      properties:
        id:
          type: string
          format: uuid
        name:
          type: string
        address:
          type: string
        created_at:
          type: string
          format: date-time

    Lot:
      type: object
      properties:
        id:
          type: string
          format: uuid
        facility_id:
          type: string
          format: uuid
        name:
          type: string

    Zone:
      type: object
      properties:
        id:
          type: string
          format: uuid
        lot_id:
          type: string
          format: uuid
        name:
          type: string
        total_stalls:
          type: integer
        hold_timeout_minutes:
          type: integer
          default: 15

    RatePlan:
      type: object
      properties:
        id:
          type: string
          format: uuid
        zone_id:
          type: string
          format: uuid
        name:
          type: string
        rate_cents:
          type: integer
        period:
          type: string
          enum: [hourly, daily, monthly]

    Member:
      type: object
      properties:
        id:
          type: string
          format: uuid
        organization_id:
          type: string
          format: uuid
        display_name:
          type: string
        arrears_balance_cents:
          type: integer
        created_at:
          type: string
          format: date-time

    Vehicle:
      type: object
      properties:
        id:
          type: string
          format: uuid
        organization_id:
          type: string
          format: uuid
        plate_number:
          type: string
        make:
          type: string
        model:
          type: string

    Driver:
      type: object
      properties:
        id:
          type: string
          format: uuid
        organization_id:
          type: string
          format: uuid
        member_id:
          type: string
          format: uuid
        licence_number:
          type: string

    MessageRule:
      type: object
      properties:
        id:
          type: string
          format: uuid
        trigger_event:
          type: string
          enum: [booking.confirmed, booking.changed, expiry.approaching, arrears.reminder]
        topic_id:
          type: string
          format: uuid
        template:
          type: string
        active:
          type: boolean

    Reservation:
      type: object
      properties:
        id:
          type: string
          format: uuid
        zone_id:
          type: string
          format: uuid
        member_id:
          type: string
          format: uuid
        vehicle_id:
          type: string
          format: uuid
        status:
          type: string
          enum: [hold, confirmed, cancelled, expired]
        time_window_start:
          type: string
          format: date-time
        time_window_end:
          type: string
          format: date-time
        stall_count:
          type: integer
        rate_plan_id:
          type: string
          format: uuid
        created_at:
          type: string
          format: date-time

    CapacityDashboard:
      type: object
      properties:
        zones:
          type: array
          items:
            type: object
            properties:
              zone_id:
                type: string
                format: uuid
              zone_name:
                type: string
              total_stalls:
                type: integer
              available_stalls:
                type: integer

    Device:
      type: object
      properties:
        id:
          type: string
          format: uuid
        device_type:
          type: string
          enum: [camera, gate, geomagnetic]
        zone_id:
          type: string
          format: uuid
        status:
          type: string
          enum: [online, offline]
        registered_at:
          type: string
          format: date-time

    DeviceEvent:
      type: object
      properties:
        id:
          type: string
          format: uuid
        device_id:
          type: string
          format: uuid
        event_key:
          type: string
        sequence_number:
          type: integer
          format: int64
        event_type:
          type: string
        payload:
          type: object
        received_at:
          type: string
          format: date-time
        late:
          type: boolean
        processed:
          type: boolean

    Exception:
      type: object
      properties:
        id:
          type: string
          format: uuid
        device_id:
          type: string
          format: uuid
        exception_type:
          type: string
          enum: [gate_stuck, sensor_offline, camera_error]
        status:
          type: string
          enum: [open, acknowledged]
        acknowledged_by:
          type: string
          format: uuid
        acknowledged_at:
          type: string
          format: date-time
        note:
          type: string
        created_at:
          type: string
          format: date-time

    Notification:
      type: object
      properties:
        id:
          type: string
          format: uuid
        topic_id:
          type: string
          format: uuid
        title:
          type: string
        body:
          type: string
        read:
          type: boolean
        dismissed:
          type: boolean
        created_at:
          type: string
          format: date-time

    DNDSettings:
      type: object
      properties:
        start_time:
          type: string
          example: "22:00"
        end_time:
          type: string
          example: "06:00"
        enabled:
          type: boolean

    Campaign:
      type: object
      properties:
        id:
          type: string
          format: uuid
        title:
          type: string
        description:
          type: string
        target_role:
          type: string
          nullable: true
        created_by:
          type: string
          format: uuid
        created_at:
          type: string
          format: date-time

    Task:
      type: object
      properties:
        id:
          type: string
          format: uuid
        campaign_id:
          type: string
          format: uuid
        description:
          type: string
        deadline:
          type: string
          format: date-time
          nullable: true
        reminder_interval_minutes:
          type: integer
        completed_at:
          type: string
          format: date-time
          nullable: true
        completed_by:
          type: string
          format: uuid
          nullable: true

    Tag:
      type: object
      properties:
        id:
          type: string
          format: uuid
        name:
          type: string

    Segment:
      type: object
      properties:
        id:
          type: string
          format: uuid
        name:
          type: string
        filter_expression:
          type: object
        schedule:
          type: string
          enum: [manual, nightly]
        created_at:
          type: string
          format: date-time

    SegmentRun:
      type: object
      properties:
        id:
          type: string
          format: uuid
        segment_id:
          type: string
          format: uuid
        ran_at:
          type: string
          format: date-time
        member_count:
          type: integer
        triggered_by:
          type: string
          enum: [manual, scheduler]

    Export:
      type: object
      properties:
        id:
          type: string
          format: uuid
        format:
          type: string
          enum: [csv, excel, pdf]
        scope:
          type: string
          enum: [occupancy, bookings, exceptions]
        status:
          type: string
          enum: [pending, ready, failed]
        created_at:
          type: string
          format: date-time
        completed_at:
          type: string
          format: date-time
          nullable: true

    AuditLog:
      type: object
      properties:
        id:
          type: string
          format: uuid
        actor_id:
          type: string
          format: uuid
        action:
          type: string
        resource_type:
          type: string
        resource_id:
          type: string
          format: uuid
        detail:
          type: object
        created_at:
          type: string
          format: date-time

security:
  - sessionCookie: []

paths:
  # ─── AUTH ───────────────────────────────────────────────────────────────────
  /auth/login:
    post:
      tags: [Auth]
      summary: Login
      security: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [username, password]
              properties:
                username:
                  type: string
                password:
                  type: string
      responses:
        "200":
          description: Login successful
          headers:
            Set-Cookie:
              schema:
                type: string
                example: session_id=abc123; HttpOnly; SameSite=Strict
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
        "401":
          description: Invalid credentials
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
        "429":
          description: Account locked
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"

  /auth/logout:
    post:
      tags: [Auth]
      summary: Logout
      responses:
        "204":
          description: Logged out

  /me:
    get:
      tags: [Auth]
      summary: Get current user
      responses:
        "200":
          description: Current user
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
        "401":
          description: Not authenticated

  /me/password:
    patch:
      tags: [Auth]
      summary: Change own password
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [current_password, new_password]
              properties:
                current_password:
                  type: string
                new_password:
                  type: string
                  minLength: 12
      responses:
        "200":
          description: Password changed
        "400":
          description: Validation error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"

  # ─── ADMIN USERS ────────────────────────────────────────────────────────────
  /admin/users:
    get:
      tags: [Admin]
      summary: List users
      parameters:
        - in: query
          name: page
          schema:
            type: integer
            default: 1
        - in: query
          name: page_size
          schema:
            type: integer
            default: 20
      responses:
        "200":
          description: User list
          content:
            application/json:
              schema:
                type: object
                properties:
                  items:
                    type: array
                    items:
                      $ref: "#/components/schemas/User"
                  total:
                    type: integer
        "403":
          description: Forbidden
    post:
      tags: [Admin]
      summary: Create user
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [username, password, display_name, roles]
              properties:
                username:
                  type: string
                password:
                  type: string
                  minLength: 12
                display_name:
                  type: string
                roles:
                  type: array
                  items:
                    type: string
      responses:
        "201":
          description: User created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
        "400":
          description: Validation error

  /admin/users/{id}:
    patch:
      tags: [Admin]
      summary: Update user
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                display_name:
                  type: string
                status:
                  type: string
                  enum: [active, inactive]
      responses:
        "200":
          description: Updated
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
    delete:
      tags: [Admin]
      summary: Delete user
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /admin/users/{id}/roles:
    patch:
      tags: [Admin]
      summary: Update user roles
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [roles]
              properties:
                roles:
                  type: array
                  items:
                    type: string
      responses:
        "200":
          description: Roles updated

  /admin/users/{id}/reset-password:
    post:
      tags: [Admin]
      summary: Admin reset user password
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [new_password]
              properties:
                new_password:
                  type: string
                  minLength: 12
      responses:
        "200":
          description: Password reset, force_password_change set to true

  /admin/users/{id}/unlock:
    post:
      tags: [Admin]
      summary: Unlock locked account
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Account unlocked

  /admin/users/{id}/sessions:
    get:
      tags: [Admin]
      summary: List active sessions for user
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Sessions
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Session"
    delete:
      tags: [Admin]
      summary: Force-expire all sessions for user
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: All sessions expired

  /admin/audit-logs:
    get:
      tags: [Admin]
      summary: Query audit log (Admin + Auditor)
      parameters:
        - in: query
          name: action
          schema:
            type: string
        - in: query
          name: actor_id
          schema:
            type: string
            format: uuid
        - in: query
          name: from
          schema:
            type: string
            format: date-time
        - in: query
          name: to
          schema:
            type: string
            format: date-time
        - in: query
          name: page
          schema:
            type: integer
            default: 1
      responses:
        "200":
          description: Audit log entries
          content:
            application/json:
              schema:
                type: object
                properties:
                  items:
                    type: array
                    items:
                      $ref: "#/components/schemas/AuditLog"
                  total:
                    type: integer

  # ─── MASTER DATA ────────────────────────────────────────────────────────────
  /facilities:
    get:
      tags: [Master Data]
      summary: List facilities
      responses:
        "200":
          description: Facilities
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Facility"
    post:
      tags: [Master Data]
      summary: Create facility
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name:
                  type: string
                address:
                  type: string
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Facility"

  /facilities/{id}:
    get:
      tags: [Master Data]
      summary: Get facility
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Facility
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Facility"
        "404":
          description: Not found
    patch:
      tags: [Master Data]
      summary: Update facility
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                address:
                  type: string
      responses:
        "200":
          description: Updated
    delete:
      tags: [Master Data]
      summary: Delete facility
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /lots:
    get:
      tags: [Master Data]
      summary: List lots
      parameters:
        - in: query
          name: facility_id
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Lots
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Lot"
    post:
      tags: [Master Data]
      summary: Create lot
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [facility_id, name]
              properties:
                facility_id:
                  type: string
                  format: uuid
                name:
                  type: string
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Lot"

  /lots/{id}:
    get:
      tags: [Master Data]
      summary: Get lot
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Lot
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Lot"
    patch:
      tags: [Master Data]
      summary: Update lot
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
      responses:
        "200":
          description: Updated
    delete:
      tags: [Master Data]
      summary: Delete lot
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /zones:
    get:
      tags: [Master Data]
      summary: List zones
      parameters:
        - in: query
          name: lot_id
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Zones
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Zone"
    post:
      tags: [Master Data]
      summary: Create zone
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [lot_id, name, total_stalls]
              properties:
                lot_id:
                  type: string
                  format: uuid
                name:
                  type: string
                total_stalls:
                  type: integer
                hold_timeout_minutes:
                  type: integer
                  default: 15
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Zone"

  /zones/{id}:
    get:
      tags: [Master Data]
      summary: Get zone
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Zone
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Zone"
    patch:
      tags: [Master Data]
      summary: Update zone
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                total_stalls:
                  type: integer
                hold_timeout_minutes:
                  type: integer
      responses:
        "200":
          description: Updated
    delete:
      tags: [Master Data]
      summary: Delete zone
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /rate-plans:
    get:
      tags: [Master Data]
      summary: List rate plans
      parameters:
        - in: query
          name: zone_id
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Rate plans
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/RatePlan"
    post:
      tags: [Master Data]
      summary: Create rate plan
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [zone_id, name, rate_cents, period]
              properties:
                zone_id:
                  type: string
                  format: uuid
                name:
                  type: string
                rate_cents:
                  type: integer
                period:
                  type: string
                  enum: [hourly, daily, monthly]
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/RatePlan"

  /rate-plans/{id}:
    get:
      tags: [Master Data]
      summary: Get rate plan
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Rate plan
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/RatePlan"
    patch:
      tags: [Master Data]
      summary: Update rate plan
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                rate_cents:
                  type: integer
                period:
                  type: string
                  enum: [hourly, daily, monthly]
      responses:
        "200":
          description: Updated
    delete:
      tags: [Master Data]
      summary: Delete rate plan
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /members:
    get:
      tags: [Master Data]
      summary: List members (org-scoped for Fleet Manager)
      parameters:
        - in: query
          name: page
          schema:
            type: integer
            default: 1
      responses:
        "200":
          description: Members
          content:
            application/json:
              schema:
                type: object
                properties:
                  items:
                    type: array
                    items:
                      $ref: "#/components/schemas/Member"
                  total:
                    type: integer
    post:
      tags: [Master Data]
      summary: Create member
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [display_name]
              properties:
                display_name:
                  type: string
                contact_notes:
                  type: string
                  description: Stored encrypted at rest
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Member"

  /members/{id}:
    get:
      tags: [Master Data]
      summary: Get member
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Member
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Member"
        "403":
          description: Cross-org access denied
    patch:
      tags: [Master Data]
      summary: Update member
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                display_name:
                  type: string
                contact_notes:
                  type: string
      responses:
        "200":
          description: Updated
    delete:
      tags: [Master Data]
      summary: Delete member
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /members/{id}/balance:
    get:
      tags: [Master Data]
      summary: Get member arrears balance
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Balance
          content:
            application/json:
              schema:
                type: object
                properties:
                  arrears_balance_cents:
                    type: integer
    patch:
      tags: [Master Data]
      summary: Adjust member arrears balance (Admin only)
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [amount_cents, reason]
              properties:
                amount_cents:
                  type: integer
                  description: Positive to increase, negative to decrease
                reason:
                  type: string
      responses:
        "200":
          description: Balance updated
        "403":
          description: Admin only

  /vehicles:
    get:
      tags: [Master Data]
      summary: List vehicles (org-scoped)
      responses:
        "200":
          description: Vehicles
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Vehicle"
    post:
      tags: [Master Data]
      summary: Create vehicle
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [plate_number]
              properties:
                plate_number:
                  type: string
                make:
                  type: string
                model:
                  type: string
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Vehicle"

  /vehicles/{id}:
    get:
      tags: [Master Data]
      summary: Get vehicle
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Vehicle
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Vehicle"
        "403":
          description: Cross-org access denied
    patch:
      tags: [Master Data]
      summary: Update vehicle
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                plate_number:
                  type: string
                make:
                  type: string
                model:
                  type: string
      responses:
        "200":
          description: Updated
    delete:
      tags: [Master Data]
      summary: Delete vehicle
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /drivers:
    get:
      tags: [Master Data]
      summary: List drivers (org-scoped)
      responses:
        "200":
          description: Drivers
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Driver"
    post:
      tags: [Master Data]
      summary: Create driver
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [member_id, licence_number]
              properties:
                member_id:
                  type: string
                  format: uuid
                licence_number:
                  type: string
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Driver"

  /drivers/{id}:
    get:
      tags: [Master Data]
      summary: Get driver
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Driver
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Driver"
    patch:
      tags: [Master Data]
      summary: Update driver
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                licence_number:
                  type: string
      responses:
        "200":
          description: Updated
    delete:
      tags: [Master Data]
      summary: Delete driver
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /message-rules:
    get:
      tags: [Master Data]
      summary: List message rules
      responses:
        "200":
          description: Message rules
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/MessageRule"
    post:
      tags: [Master Data]
      summary: Create message rule
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [trigger_event, topic_id, template]
              properties:
                trigger_event:
                  type: string
                  enum: [booking.confirmed, booking.changed, expiry.approaching, arrears.reminder]
                topic_id:
                  type: string
                  format: uuid
                template:
                  type: string
                active:
                  type: boolean
                  default: true
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/MessageRule"

  /message-rules/{id}:
    patch:
      tags: [Master Data]
      summary: Update message rule
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                template:
                  type: string
                active:
                  type: boolean
      responses:
        "200":
          description: Updated
    delete:
      tags: [Master Data]
      summary: Delete message rule
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  # ─── RESERVATIONS ───────────────────────────────────────────────────────────
  /availability:
    get:
      tags: [Reservations]
      summary: Check stall availability for a zone and time window
      parameters:
        - in: query
          name: zone_id
          required: true
          schema:
            type: string
            format: uuid
        - in: query
          name: start
          required: true
          schema:
            type: string
            format: date-time
        - in: query
          name: end
          required: true
          schema:
            type: string
            format: date-time
      responses:
        "200":
          description: Availability
          content:
            application/json:
              schema:
                type: object
                properties:
                  zone_id:
                    type: string
                    format: uuid
                  available_stalls:
                    type: integer
                  conflict:
                    type: boolean

  /reservations/hold:
    post:
      tags: [Reservations]
      summary: Create capacity hold
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [zone_id, member_id, time_window_start, time_window_end, stall_count]
              properties:
                zone_id:
                  type: string
                  format: uuid
                member_id:
                  type: string
                  format: uuid
                vehicle_id:
                  type: string
                  format: uuid
                time_window_start:
                  type: string
                  format: date-time
                time_window_end:
                  type: string
                  format: date-time
                stall_count:
                  type: integer
                rate_plan_id:
                  type: string
                  format: uuid
      responses:
        "201":
          description: Hold created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Reservation"
        "409":
          description: No stalls available
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"

  /reservations:
    get:
      tags: [Reservations]
      summary: List reservations
      parameters:
        - in: query
          name: zone_id
          schema:
            type: string
            format: uuid
        - in: query
          name: status
          schema:
            type: string
            enum: [hold, confirmed, cancelled, expired]
        - in: query
          name: from
          schema:
            type: string
            format: date-time
        - in: query
          name: to
          schema:
            type: string
            format: date-time
        - in: query
          name: page
          schema:
            type: integer
            default: 1
      responses:
        "200":
          description: Reservations
          content:
            application/json:
              schema:
                type: object
                properties:
                  items:
                    type: array
                    items:
                      $ref: "#/components/schemas/Reservation"
                  total:
                    type: integer
    post:
      tags: [Reservations]
      summary: Create reservation directly (without separate hold step)
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [zone_id, member_id, time_window_start, time_window_end, stall_count]
              properties:
                zone_id:
                  type: string
                  format: uuid
                member_id:
                  type: string
                  format: uuid
                vehicle_id:
                  type: string
                  format: uuid
                time_window_start:
                  type: string
                  format: date-time
                time_window_end:
                  type: string
                  format: date-time
                stall_count:
                  type: integer
                rate_plan_id:
                  type: string
                  format: uuid
      responses:
        "201":
          description: Reservation created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Reservation"
        "409":
          description: No stalls available

  /reservations/{id}:
    get:
      tags: [Reservations]
      summary: Get reservation
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Reservation
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Reservation"
    patch:
      tags: [Reservations]
      summary: Update reservation (hold or confirmed state only)
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                time_window_start:
                  type: string
                  format: date-time
                time_window_end:
                  type: string
                  format: date-time
                stall_count:
                  type: integer
                notes:
                  type: string
      responses:
        "200":
          description: Updated
        "409":
          description: Cannot update in current state

  /reservations/{id}/confirm:
    post:
      tags: [Reservations]
      summary: Confirm reservation
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Confirmed
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Reservation"
        "409":
          description: Hold expired or already confirmed

  /reservations/{id}/cancel:
    post:
      tags: [Reservations]
      summary: Cancel reservation
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                reason:
                  type: string
      responses:
        "200":
          description: Cancelled
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Reservation"

  /reservations/{id}/timeline:
    get:
      tags: [Reservations]
      summary: Get booking event timeline
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Timeline events
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    event_type:
                      type: string
                    occurred_at:
                      type: string
                      format: date-time
                    actor_id:
                      type: string
                      format: uuid

  # ─── CAPACITY ───────────────────────────────────────────────────────────────
  /capacity/dashboard:
    get:
      tags: [Capacity]
      summary: Capacity dashboard — all zones current availability
      responses:
        "200":
          description: Dashboard data
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/CapacityDashboard"

  /capacity/zones/{id}/stalls:
    get:
      tags: [Capacity]
      summary: Remaining stalls for a zone in a time window
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
        - in: query
          name: start
          required: true
          schema:
            type: string
            format: date-time
        - in: query
          name: end
          required: true
          schema:
            type: string
            format: date-time
      responses:
        "200":
          description: Stall availability
          content:
            application/json:
              schema:
                type: object
                properties:
                  zone_id:
                    type: string
                    format: uuid
                  available_stalls:
                    type: integer

  /capacity/snapshots:
    get:
      tags: [Capacity]
      summary: List capacity snapshots
      parameters:
        - in: query
          name: zone_id
          schema:
            type: string
            format: uuid
        - in: query
          name: from
          schema:
            type: string
            format: date-time
      responses:
        "200":
          description: Snapshots
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    id:
                      type: string
                      format: uuid
                    zone_id:
                      type: string
                      format: uuid
                    snapshot_at:
                      type: string
                      format: date-time
                    authoritative_stalls:
                      type: integer

  # ─── DEVICES ────────────────────────────────────────────────────────────────
  /devices:
    get:
      tags: [Devices]
      summary: List devices
      responses:
        "200":
          description: Devices
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Device"
    post:
      tags: [Devices]
      summary: Register device
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [device_type, zone_id]
              properties:
                device_type:
                  type: string
                  enum: [camera, gate, geomagnetic]
                zone_id:
                  type: string
                  format: uuid
      responses:
        "201":
          description: Device registered
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Device"

  /devices/{id}:
    get:
      tags: [Devices]
      summary: Get device
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Device
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Device"
    patch:
      tags: [Devices]
      summary: Update device
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                zone_id:
                  type: string
                  format: uuid
                status:
                  type: string
                  enum: [online, offline]
      responses:
        "200":
          description: Updated
    delete:
      tags: [Devices]
      summary: Delete device
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /device-events:
    get:
      tags: [Devices]
      summary: List device events
      parameters:
        - in: query
          name: device_id
          schema:
            type: string
            format: uuid
        - in: query
          name: late
          schema:
            type: boolean
        - in: query
          name: from
          schema:
            type: string
            format: date-time
        - in: query
          name: to
          schema:
            type: string
            format: date-time
        - in: query
          name: page
          schema:
            type: integer
            default: 1
      responses:
        "200":
          description: Device events
          content:
            application/json:
              schema:
                type: object
                properties:
                  items:
                    type: array
                    items:
                      $ref: "#/components/schemas/DeviceEvent"
                  total:
                    type: integer
    post:
      tags: [Devices]
      summary: Ingest device event
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [device_id, event_key, sequence_number, event_type]
              properties:
                device_id:
                  type: string
                  format: uuid
                event_key:
                  type: string
                sequence_number:
                  type: integer
                  format: int64
                event_type:
                  type: string
                payload:
                  type: object
                device_time:
                  type: string
                  format: date-time
                device_time_signature:
                  type: string
                  description: HMAC-SHA256 of payload using pre-shared device key
      responses:
        "201":
          description: Event accepted
        "200":
          description: Duplicate event key — idempotent, not reprocessed
        "400":
          description: Missing required fields
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"

  /device-events/{id}:
    get:
      tags: [Devices]
      summary: Get device event
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Device event
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/DeviceEvent"

  /device-events/replay:
    post:
      tags: [Devices]
      summary: Replay device events
      description: Replays events by event key. Already-processed keys are skipped without double-counting.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [event_keys]
              properties:
                event_keys:
                  type: array
                  items:
                    type: string
      responses:
        "200":
          description: Replay result
          content:
            application/json:
              schema:
                type: object
                properties:
                  replayed:
                    type: integer
                  skipped:
                    type: integer

  # ─── EXCEPTIONS ─────────────────────────────────────────────────────────────
  /exceptions:
    get:
      tags: [Exceptions]
      summary: List open exceptions
      parameters:
        - in: query
          name: status
          schema:
            type: string
            enum: [open, acknowledged]
            default: open
      responses:
        "200":
          description: Exceptions
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Exception"

  /exceptions/history:
    get:
      tags: [Exceptions]
      summary: Exception history (acknowledged)
      parameters:
        - in: query
          name: from
          schema:
            type: string
            format: date-time
        - in: query
          name: page
          schema:
            type: integer
            default: 1
      responses:
        "200":
          description: Exception history
          content:
            application/json:
              schema:
                type: object
                properties:
                  items:
                    type: array
                    items:
                      $ref: "#/components/schemas/Exception"
                  total:
                    type: integer

  /exceptions/{id}:
    get:
      tags: [Exceptions]
      summary: Get exception
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Exception
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Exception"

  /exceptions/{id}/acknowledge:
    post:
      tags: [Exceptions]
      summary: Acknowledge exception (Dispatch Operator)
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                note:
                  type: string
      responses:
        "200":
          description: Acknowledged
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Exception"
        "403":
          description: Dispatch Operator role required

  # ─── TRACKING ───────────────────────────────────────────────────────────────
  /tracking/location:
    post:
      tags: [Tracking]
      summary: Submit vehicle location report
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [vehicle_id, latitude, longitude]
              properties:
                vehicle_id:
                  type: string
                  format: uuid
                latitude:
                  type: number
                  format: double
                longitude:
                  type: number
                  format: double
                device_time:
                  type: string
                  format: date-time
                device_time_signature:
                  type: string
      responses:
        "201":
          description: Location recorded

  /tracking/vehicles/{id}/positions:
    get:
      tags: [Tracking]
      summary: Get vehicle position history
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
        - in: query
          name: from
          schema:
            type: string
            format: date-time
        - in: query
          name: to
          schema:
            type: string
            format: date-time
      responses:
        "200":
          description: Positions
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    latitude:
                      type: number
                    longitude:
                      type: number
                    received_at:
                      type: string
                      format: date-time
                    suspect:
                      type: boolean

  /tracking/vehicles/{id}/stops:
    get:
      tags: [Tracking]
      summary: Get vehicle stop events
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Stop events
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    started_at:
                      type: string
                      format: date-time
                    detected_at:
                      type: string
                      format: date-time
                    latitude:
                      type: number
                    longitude:
                      type: number

  # ─── NOTIFICATIONS ──────────────────────────────────────────────────────────
  /notifications:
    get:
      tags: [Notifications]
      summary: List notifications for current user
      parameters:
        - in: query
          name: read
          schema:
            type: boolean
        - in: query
          name: page
          schema:
            type: integer
            default: 1
      responses:
        "200":
          description: Notifications
          content:
            application/json:
              schema:
                type: object
                properties:
                  items:
                    type: array
                    items:
                      $ref: "#/components/schemas/Notification"
                  total:
                    type: integer

  /notifications/{id}/read:
    patch:
      tags: [Notifications]
      summary: Mark notification as read
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Marked read

  /notifications/{id}/dismiss:
    post:
      tags: [Notifications]
      summary: Dismiss notification
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Dismissed

  /notification-topics:
    get:
      tags: [Notifications]
      summary: List notification topics
      responses:
        "200":
          description: Topics
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    id:
                      type: string
                      format: uuid
                    name:
                      type: string
                    subscribed:
                      type: boolean

  /notification-topics/{id}/subscribe:
    post:
      tags: [Notifications]
      summary: Subscribe to topic
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Subscribed
    delete:
      tags: [Notifications]
      summary: Unsubscribe from topic
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Unsubscribed

  /notification-settings:
    get:
      tags: [Notifications]
      summary: Get notification settings
      responses:
        "200":
          description: Settings
          content:
            application/json:
              schema:
                type: object
                properties:
                  dnd:
                    $ref: "#/components/schemas/DNDSettings"
    patch:
      tags: [Notifications]
      summary: Update notification settings
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                dnd:
                  $ref: "#/components/schemas/DNDSettings"
      responses:
        "200":
          description: Updated

  /notification-settings/dnd:
    get:
      tags: [Notifications]
      summary: Get DND settings
      responses:
        "200":
          description: DND settings
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/DNDSettings"
    patch:
      tags: [Notifications]
      summary: Update DND settings
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/DNDSettings"
      responses:
        "200":
          description: Updated

  /notifications/export-packages:
    get:
      tags: [Notifications]
      summary: List SMS/email export packages
      responses:
        "200":
          description: Export packages
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    id:
                      type: string
                      format: uuid
                    channel:
                      type: string
                      enum: [sms, email]
                    recipient:
                      type: string
                    created_at:
                      type: string
                      format: date-time
                    downloaded_at:
                      type: string
                      format: date-time
                      nullable: true

  /notifications/export-packages/{id}/download:
    get:
      tags: [Notifications]
      summary: Download SMS/email export package
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Package file
          content:
            application/json:
              schema:
                type: object

  # ─── CAMPAIGNS AND TASKS ────────────────────────────────────────────────────
  /campaigns:
    get:
      tags: [Campaigns]
      summary: List campaigns
      responses:
        "200":
          description: Campaigns
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Campaign"
    post:
      tags: [Campaigns]
      summary: Create campaign
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [title]
              properties:
                title:
                  type: string
                description:
                  type: string
                target_role:
                  type: string
                  nullable: true
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Campaign"

  /campaigns/{id}:
    get:
      tags: [Campaigns]
      summary: Get campaign
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Campaign
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Campaign"
    patch:
      tags: [Campaigns]
      summary: Update campaign
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                title:
                  type: string
                description:
                  type: string
      responses:
        "200":
          description: Updated
    delete:
      tags: [Campaigns]
      summary: Delete campaign
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /campaigns/{id}/tasks:
    get:
      tags: [Campaigns]
      summary: List tasks for campaign
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Tasks
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Task"
    post:
      tags: [Campaigns]
      summary: Create task in campaign
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [description]
              properties:
                description:
                  type: string
                deadline:
                  type: string
                  format: date-time
                reminder_interval_minutes:
                  type: integer
                  default: 60
      responses:
        "201":
          description: Task created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Task"

  /tasks/{id}:
    get:
      tags: [Campaigns]
      summary: Get task
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Task
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Task"
    patch:
      tags: [Campaigns]
      summary: Update task
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                description:
                  type: string
                deadline:
                  type: string
                  format: date-time
                reminder_interval_minutes:
                  type: integer
      responses:
        "200":
          description: Updated
    delete:
      tags: [Campaigns]
      summary: Delete task
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /tasks/{id}/complete:
    post:
      tags: [Campaigns]
      summary: Mark task complete
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Task marked complete
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Task"

  # ─── TAGS AND SEGMENTS ──────────────────────────────────────────────────────
  /tags:
    get:
      tags: [Segmentation]
      summary: List tags
      responses:
        "200":
          description: Tags
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Tag"
    post:
      tags: [Segmentation]
      summary: Create tag
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name:
                  type: string
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Tag"

  /tags/{id}:
    delete:
      tags: [Segmentation]
      summary: Delete tag
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /members/{id}/tags:
    get:
      tags: [Segmentation]
      summary: Get tags for member
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Member tags
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Tag"
    post:
      tags: [Segmentation]
      summary: Add tag to member
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [tag_id]
              properties:
                tag_id:
                  type: string
                  format: uuid
      responses:
        "200":
          description: Tag added

  /members/{id}/tags/{tagId}:
    delete:
      tags: [Segmentation]
      summary: Remove tag from member
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
        - in: path
          name: tagId
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Tag removed

  /tags/export:
    post:
      tags: [Segmentation]
      summary: Export tag version snapshot
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                member_ids:
                  type: array
                  items:
                    type: string
                    format: uuid
                  description: Leave empty to export all members
      responses:
        "200":
          description: Tag version snapshot
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
                    format: uuid
                  exported_at:
                    type: string
                    format: date-time
                  snapshot:
                    type: object

  /tags/import:
    post:
      tags: [Segmentation]
      summary: Import tag version snapshot (restore)
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [snapshot]
              properties:
                snapshot:
                  type: object
                  description: Previously exported snapshot object
      responses:
        "200":
          description: Tags restored, audit log entry written

  /segments:
    get:
      tags: [Segmentation]
      summary: List segments
      responses:
        "200":
          description: Segments
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Segment"
    post:
      tags: [Segmentation]
      summary: Create segment
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name, filter_expression]
              properties:
                name:
                  type: string
                filter_expression:
                  type: object
                  description: |
                    Structured filter. Example:
                    {"and": [{"tag": "downtown_monthly"}, {"arrears_balance_cents": {"gt": 5000}}]}
                schedule:
                  type: string
                  enum: [manual, nightly]
                  default: manual
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Segment"

  /segments/{id}:
    get:
      tags: [Segmentation]
      summary: Get segment
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Segment
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Segment"
    patch:
      tags: [Segmentation]
      summary: Update segment
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                filter_expression:
                  type: object
                schedule:
                  type: string
                  enum: [manual, nightly]
      responses:
        "200":
          description: Updated
    delete:
      tags: [Segmentation]
      summary: Delete segment
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "204":
          description: Deleted

  /segments/{id}/preview:
    post:
      tags: [Segmentation]
      summary: Preview segment — returns matching member count without activating
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Preview count
          content:
            application/json:
              schema:
                type: object
                properties:
                  member_count:
                    type: integer

  /segments/{id}/run:
    post:
      tags: [Segmentation]
      summary: Run segment on-demand
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Run result
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/SegmentRun"

  /segments/{id}/runs:
    get:
      tags: [Segmentation]
      summary: Get segment run history
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Run history
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/SegmentRun"

  # ─── ANALYTICS AND EXPORTS ──────────────────────────────────────────────────
  /analytics/occupancy:
    get:
      tags: [Analytics]
      summary: Occupancy trend data
      parameters:
        - in: query
          name: zone_id
          schema:
            type: string
            format: uuid
        - in: query
          name: from
          required: true
          schema:
            type: string
            format: date-time
        - in: query
          name: to
          required: true
          schema:
            type: string
            format: date-time
        - in: query
          name: granularity
          schema:
            type: string
            enum: [hour, day, week]
            default: day
      responses:
        "200":
          description: Occupancy trend
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    period:
                      type: string
                    avg_occupancy_pct:
                      type: number
                    peak_occupancy_pct:
                      type: number

  /analytics/bookings:
    get:
      tags: [Analytics]
      summary: Booking distribution pivot
      parameters:
        - in: query
          name: pivot_by
          schema:
            type: string
            enum: [time, region, category, entity, risk_level]
            default: time
        - in: query
          name: from
          schema:
            type: string
            format: date-time
        - in: query
          name: to
          schema:
            type: string
            format: date-time
      responses:
        "200":
          description: Booking distribution
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    label:
                      type: string
                    count:
                      type: integer
                    total_stalls:
                      type: integer

  /analytics/exceptions:
    get:
      tags: [Analytics]
      summary: Device exception trends
      parameters:
        - in: query
          name: from
          schema:
            type: string
            format: date-time
        - in: query
          name: to
          schema:
            type: string
            format: date-time
      responses:
        "200":
          description: Exception trends
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    exception_type:
                      type: string
                    count:
                      type: integer

  /exports:
    get:
      tags: [Analytics]
      summary: List past exports
      responses:
        "200":
          description: Exports
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Export"
    post:
      tags: [Analytics]
      summary: Generate export
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [format, scope]
              properties:
                format:
                  type: string
                  enum: [csv, excel, pdf]
                scope:
                  type: string
                  enum: [occupancy, bookings, exceptions]
                segment_id:
                  type: string
                  format: uuid
                  nullable: true
                  description: If set, user must be a member of this segment
                from:
                  type: string
                  format: date-time
                to:
                  type: string
                  format: date-time
      responses:
        "201":
          description: Export queued
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Export"
        "403":
          description: Role or segment access denied

  /exports/{id}/download:
    get:
      tags: [Analytics]
      summary: Download export file
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Export file
          content:
            text/csv:
              schema:
                type: string
            application/vnd.openxmlformats-officedocument.spreadsheetml.sheet:
              schema:
                type: string
                format: binary
            application/pdf:
              schema:
                type: string
                format: binary
        "404":
          description: Export not found or not ready

tags:
  - name: Auth
    description: Authentication and session management
  - name: Admin
    description: Admin user management and audit log (Admin role required)
  - name: Master Data
    description: Facilities, lots, zones, rate plans, members, vehicles, drivers, message rules
  - name: Reservations
    description: Reservation lifecycle and capacity management
  - name: Capacity
    description: Capacity dashboard and snapshots
  - name: Devices
    description: Device registration, event ingestion, and replay
  - name: Exceptions
    description: Device exception monitoring and acknowledgement
  - name: Tracking
    description: Real-time vehicle location and stop detection
  - name: Notifications
    description: Notification subscriptions, delivery, DND, and export packages
  - name: Campaigns
    description: Campaigns and tasks with deadline reminders
  - name: Segmentation
    description: Member tagging, segment definitions, and tag version management
  - name: Analytics
    description: Pivot analytics, trend charts, and data exports
