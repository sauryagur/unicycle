# Unicycle — Campus Bicycle Sharing System

### Technical Specification v1.0

**Thapar Institute of Engineering & Technology**
_Authored for internal development use_

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [System Architecture](#2-system-architecture)
3. [Hardware Design](#3-hardware-design)
4. [Communication Protocols](#4-communication-protocols)
5. [Firmware Design](#5-firmware-design)
6. [Backend Services](#6-backend-services)
7. [Mobile Application](#7-mobile-application)
8. [Security Model](#8-security-model)
9. [Billing and Wallet System](#9-billing-and-wallet-system)
10. [Fault Tolerance and Edge Cases](#10-fault-tolerance-and-edge-cases)
11. [Observability and Operations](#11-observability-and-operations)
12. [Deployment](#12-deployment)
13. [Ride State Machine](#13-ride-state-machine)
14. [FAQ](#14-faq)

---

## 1. Project Overview

Unicycle is a campuswide bicycle sharing system designed for Thapar Institute of Engineering & Technology. Students rent bicycles using a mobile app, unlock them by scanning a QR code, and return them to any designated parking station on campus. The system is designed to be self-contained, fault-tolerant, and operable entirely within campus jurisdiction.

### Goals

- Provide convenient, affordable short-distance transport across campus
- Require zero manual intervention for normal ride operations
- Degrade gracefully under hardware failure or network interruption
- Give operations staff real-time visibility into fleet health
- Enforce accountability through Thapar student ID binding

### Non-Goals

- Off-campus operation
- GPS tracking on bicycles (phone-based geofencing is used instead)
- Third-party payment gateways (campus wallet system only)
- Support for non-Thapar users

### Glossary

| Term            | Definition                                                                         |
| --------------- | ---------------------------------------------------------------------------------- |
| Bicycle node    | An ESP32 mounted on a bicycle with a solar panel, latch mechanism, LED, and buzzer |
| Router node     | An ESP32 with a 4G SIM card installed at a parking station                         |
| Parking station | A designated area with bicycle stands and at least one router node                 |
| Ride session    | The period between a confirmed unlock and a confirmed lock                         |
| Orange state    | Bicycle is locked but cannot reach a router to confirm ride end                    |
| Red state       | Bicycle has confirmed ride end with a router; billing stops                        |

---

## 2. System Architecture

The system is composed of four layers:

```
┌─────────────────────────────────────────────┐
│               Mobile App (Flutter)           │
│         User-facing: QR scan, wallet         │
└────────────────────┬────────────────────────┘
                     │ HTTPS / WebSocket
┌────────────────────▼────────────────────────┐
│             Backend Services                 │
│   Go (real-time)  │  Hono/TS (REST/auth)    │
│         PostgreSQL + TimescaleDB             │
│       Redis pub/sub │ Mosquitto MQTT         │
└────────────────────┬────────────────────────┘
                     │ MQTT over 4G
┌────────────────────▼────────────────────────┐
│           Router Nodes (ESP32 + SIM)         │
│        One per parking station               │
└────────────────────┬────────────────────────┘
                     │ ESP-NOW
┌────────────────────▼────────────────────────┐
│            Bicycle Nodes (ESP32)             │
│     Solar-powered, local state, NVS log      │
└─────────────────────────────────────────────┘
```

### Data Flow — Unlock

1. User scans QR code on bicycle in the app
2. App sends `{bike_uuid, user_session_token}` to Hono API over HTTPS
3. Hono validates session, checks bike availability and battery threshold
4. Hono publishes unlock command to Redis; Go service receives it
5. Go service publishes `commands/{bike_id}` to Mosquitto
6. Router node subscribed to that topic receives the command over 4G/MQTT
7. Router node forwards command to bicycle via ESP-NOW
8. Bicycle verifies ECDSA signature, actuates electromagnet, sends confirmation
9. Confirmation travels back: bicycle → router (ESP-NOW) → server (MQTT) → app (WebSocket)
10. App shows green LED state; timer starts on both server and bicycle

### Data Flow — Lock and Ride End

1. User rotates latch on bicycle (physical action)
2. Bicycle detects latch state change, sends lock event to router via ESP-NOW
3. Router forwards to server via MQTT
4. Server records ride end timestamp, debits wallet, marks bicycle available
5. Server sends confirmation back to bicycle via same path
6. Bicycle LED turns red; app receives WebSocket notification

---

## 3. Hardware Design

### 3.1 Bicycle Node

| Component       | Part                             | Notes                                                  |
| --------------- | -------------------------------- | ------------------------------------------------------ |
| Microcontroller | ESP32-WROOM-32                   | Dual-core, built-in Wi-Fi/BLE radio used for ESP-NOW   |
| Solar panel     | 5V/1W panel                      | Sufficient for standby current with periodic activity  |
| Battery         | 18650 Li-ion, 3000mAh            | Managed via TP4056 charge controller                   |
| Latch mechanism | Solenoid-actuated rotating latch | Electromagnet unlocks; spring or manual rotation locks |
| Electromagnet   | 12V DC push-pull solenoid        | Driven via MOSFET gate from ESP32 GPIO                 |
| LED indicator   | RGB LED (common cathode)         | Green / Orange / Red states                            |
| Buzzer          | Passive piezo buzzer             | Driven via PWM for frequency-variable beeping          |
| Accelerometer   | MPU-6050 (I2C)                   | Detects upright orientation, impact events             |
| RTC             | DS3231 (I2C)                     | Maintains accurate time when ESP32 is in deep sleep    |
| Storage         | ESP32 NVS (flash)                | Append-only local event log, ~50KB reserved            |
| Power switch    | Physical power button            | For maintenance only                                   |

**Power budget (standby):** ~8mA average with ESP-NOW radio in light sleep, periodic wakeup every 5 seconds to send heartbeat. The 5V/1W panel provides ~200mA at peak sun; sufficient to maintain charge through a normal campus day.

**Battery threshold:** Bicycles below 15% state-of-charge cannot be unlocked. This threshold is reported in every heartbeat and enforced by both the server and the bicycle firmware locally.

### 3.2 Router Node

| Component       | Part                  | Notes                                          |
| --------------- | --------------------- | ---------------------------------------------- |
| Microcontroller | ESP32-WROOM-32        | ESP-NOW for bike communication                 |
| 4G modem        | SIM7600E              | UART connection to ESP32, AT command interface |
| SIM card        | Campus IoT SIM        | Data-only plan, fixed IP preferred             |
| Power           | Mains AC adapter      | Router nodes are stationary and mains-powered  |
| Enclosure       | IP65 weatherproof box | Outdoor rated                                  |
| Antenna         | External 4G antenna   | Mounted on enclosure for signal strength       |

**Placement:** One router node per parking station, mounted at ~2m height on a pole or wall adjacent to the bicycle stands. Each router node covers approximately 50–100m radius for ESP-NOW communication.

### 3.3 LED States

| Color          | Meaning                                  | User Action                            |
| -------------- | ---------------------------------------- | -------------------------------------- |
| Off            | Bicycle not in use or sleeping           | —                                      |
| Green          | Unlocked, ride in progress               | Begin riding                           |
| Orange         | Locked, seeking router confirmation      | Wait or move closer to parking station |
| Red            | Locked, ride confirmed ended             | Walk away; billing has stopped         |
| Slow red blink | Battery low warning                      | Return soon                            |
| Fast red blink | Battery critical; forced return imminent | Return immediately                     |

---

## 4. Communication Protocols

### 4.1 ESP-NOW (Bicycle ↔ Router)

ESP-NOW is Espressif's connectionless, low-latency peer-to-peer protocol built on the 802.11 physical layer. It requires no Wi-Fi association, has ~1ms latency, and works up to 200m in open space.

**Design choices:**

- Each router node stores the MAC addresses of bicycles it has seen in the last 60 seconds as its "connected bike" list
- Bicycles broadcast a heartbeat frame every 5 seconds containing `{bike_id, battery_pct, ride_state, latch_state, timestamp}`
- Routers unicast command frames to specific bicycle MAC addresses
- All frames are signed; unsigned frames are silently dropped

**Frame types:**

| Type           | Direction     | Payload                                                                 |
| -------------- | ------------- | ----------------------------------------------------------------------- |
| `HEARTBEAT`    | Bike → Router | bike_id, battery, state, timestamp, accelerometer_upright               |
| `UNLOCK_CMD`   | Router → Bike | bike_id, user_id, session_token_hash, server_timestamp, ECDSA signature |
| `LOCK_CONFIRM` | Bike → Router | bike_id, ride_id, end_timestamp, local_log_hash, ECDSA signature        |
| `ACK`          | Router → Bike | ride_id, server_timestamp                                               |
| `LOG_SYNC`     | Bike → Router | bike_id, log_entries[] (batched on reconnect)                           |

### 4.2 MQTT (Router ↔ Server)

Mosquitto broker hosted on the backend VPS. Routers connect as persistent MQTT clients using their router ID as the client ID.

**Topic structure:**

```
bikes/{bike_id}/heartbeat         # router publishes bike telemetry
bikes/{bike_id}/events            # lock, unlock confirmations, errors
bikes/{bike_id}/log_sync          # offline log uploads
routers/{router_id}/status        # router heartbeat (online/offline)
routers/{router_id}/bikes         # current connected bike list
commands/{bike_id}                # server → router → bike commands
```

**QoS levels:**

- Heartbeats: QoS 0 (fire and forget; loss is acceptable)
- Commands and confirmations: QoS 1 (at least once; idempotency handled at application layer)
- Log sync: QoS 1

### 4.3 HTTPS / WebSocket (Server ↔ App)

- REST endpoints served by the Hono service over HTTPS (TLS via Caddy)
- Real-time ride state pushed to the app over WebSocket from the Go service
- WebSocket connection authenticated via JWT issued at OAuth login

---

## 5. Firmware Design

### 5.1 Framework

**ESP-IDF** (not Arduino). Reasons: direct FreeRTOS access, fine-grained power management APIs, mbedTLS built in, ESP-NOW and NVS first-class support, no Arduino HAL overhead.

### 5.2 Task Architecture (Bicycle Node)

```
┌─────────────────────────────────────────────────┐
│                  FreeRTOS Scheduler              │
├──────────────┬──────────────┬────────────────────┤
│  comms_task  │  latch_task  │    power_task       │
│  (ESP-NOW    │  (electromagnet│  (ADC battery,    │
│  send/recv)  │  LED, buzzer) │  sleep decisions)  │
└──────┬───────┴──────┬───────┴─────────────────────┘
       │              │
       ▼              ▼
   ┌───────────────────────┐
   │       log_task        │
   │  (NVS append-only     │
   │   event log writer)   │
   └───────────────────────┘

Inter-task communication: FreeRTOS queues only. No shared global state.
```

**Task responsibilities:**

- `comms_task`: Sends heartbeats every 5s, receives and validates incoming frames, dispatches unlock commands to `latch_task` via queue, sends lock confirmations
- `latch_task`: Owns LED and buzzer state machine, actuates electromagnet on valid unlock command, detects latch sensor state changes, sends events to `comms_task`
- `power_task`: Reads battery ADC every 30s, manages ESP-NOW radio sleep/wake, triggers deep sleep if battery is critically low, enforces battery threshold
- `log_task`: Receives events from all other tasks, appends to NVS with sequence number and timestamp, manages log rotation, batches log entries for sync

### 5.3 Local State Machine

The bicycle maintains its own ride state independently. See Section 13 for the full state machine. Key invariants:

- The bicycle **never** unlocks without a cryptographically valid command
- The bicycle **always** records events to NVS before acting on them
- Timer and balance are synced from server at ride start and run locally thereafter
- Balance exhaustion triggers escalating beep warnings; it does not force-lock the bicycle

### 5.4 Offline Log Format

Each NVS log entry is a fixed-size struct:

```c
typedef struct {
    uint32_t seq;           // monotonic sequence number
    int64_t  timestamp_ms;  // local RTC time
    uint8_t  event_type;    // enum: UNLOCK, LOCK, HEARTBEAT, ERROR, etc.
    uint8_t  ride_state;    // state at time of event
    uint16_t battery_pct;   // battery percentage * 100
    uint8_t  payload[32];   // event-specific data (ride_id, user_id hash, etc.)
    uint8_t  prev_hash[8];  // truncated SHA-256 of previous entry (chain integrity)
} log_entry_t;
```

Entries are chained using a truncated hash of the previous entry. When a log syncs to the server, the chain is validated to detect tampering. The server reconciles synced logs against its own ride records and resolves discrepancies in favor of the server timestamp unless the delta is within clock drift tolerance (±5 seconds).

### 5.5 Router Node Firmware

The router node runs two concurrent tasks:

- `espnow_task`: Maintains connected bike table, forwards frames between ESP-NOW and MQTT
- `mqtt_task`: Maintains persistent MQTT connection, publishes bike telemetry, receives server commands and forwards to target bike via ESP-NOW

The router has no business logic. It is a relay. The connected bike table is a simple `{mac_address → last_seen_ms}` map; entries expire after 15 seconds without a heartbeat.

---

## 6. Backend Services

### 6.1 Service Split

Two services with distinct responsibilities:

| Service        | Language                | Responsibility                                                        |
| -------------- | ----------------------- | --------------------------------------------------------------------- |
| `unicycle-rt`  | Go                      | MQTT client, WebSocket server, ride session management, state machine |
| `unicycle-api` | Hono / TypeScript (Bun) | REST API, OAuth, wallet, ride history, admin endpoints, metrics       |

They communicate via **Redis pub/sub**. `unicycle-rt` publishes ride state change events; `unicycle-api` subscribes to update PostgreSQL and trigger push notifications.

### 6.2 unicycle-rt (Go)

**Key packages:** `eclipse/paho.mqtt.golang`, `gorilla/websocket`, `prometheus/client_golang`

**Concurrency model:** One goroutine per active ride. Each ride goroutine owns:

- A ticker for billing increments
- A context with cancellation (cancelled on ride end or timeout)
- A channel receiving bike events from the MQTT subscriber
- The authoritative ride state

When a ride ends, the goroutine publishes a `ride_ended` event to Redis and exits cleanly.

**Network state model:** `unicycle-rt` maintains an in-memory model of the current fleet:

```go
type FleetState struct {
    Bikes   map[string]*BikeState   // bike_id → state
    Routers map[string]*RouterState // router_id → state
    mu      sync.RWMutex
}
```

This model is updated from MQTT heartbeats and is the source of truth for the admin dashboard's real-time view. It is not persisted — it is rebuilt from incoming MQTT messages on service restart.

### 6.3 unicycle-api (Hono / TypeScript)

**Runtime:** Bun

**Key routes:**

```
POST   /auth/google              # Google OAuth callback, issues JWT
GET    /bikes                    # List available bikes with battery status
POST   /rides/start              # Validate session, reserve bike, trigger unlock
GET    /rides/:id                # Ride details
GET    /rides/history            # User's past rides
POST   /wallet/topup             # Add funds to wallet
GET    /wallet/balance           # Current balance
POST   /bikes/:id/report         # Report damage or issue
GET    /admin/fleet              # Fleet state (admin only)
GET    /admin/routers            # Router health grid (admin only)
POST   /admin/bikes/:id/disable  # Manually disable a bike (admin only)
```

All routes except `/auth/google` require a valid JWT. Admin routes additionally require an `admin` role claim in the JWT.

### 6.4 Database Schema (PostgreSQL)

```sql
-- Users
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thapar_id   TEXT UNIQUE NOT NULL,
    email       TEXT UNIQUE NOT NULL,
    google_sub  TEXT UNIQUE NOT NULL,
    wallet_paise INTEGER NOT NULL DEFAULT 0,  -- stored in paise (1/100 rupee)
    suspended   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Bicycles
CREATE TABLE bikes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mac_address     MACADDR UNIQUE NOT NULL,
    serial_number   TEXT UNIQUE NOT NULL,
    public_key      BYTEA NOT NULL,  -- ECDSA public key for this bike
    state           TEXT NOT NULL DEFAULT 'available',
    current_ride_id UUID REFERENCES rides(id),
    battery_pct     SMALLINT,
    last_seen_at    TIMESTAMPTZ,
    disabled        BOOLEAN NOT NULL DEFAULT false
);

-- Router nodes
CREATE TABLE routers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    location_name   TEXT NOT NULL,
    last_seen_at    TIMESTAMPTZ,
    online          BOOLEAN NOT NULL DEFAULT false
);

-- Rides
CREATE TABLE rides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    bike_id         UUID NOT NULL REFERENCES bikes(id),
    started_at      TIMESTAMPTZ NOT NULL,
    ended_at        TIMESTAMPTZ,
    start_router_id UUID REFERENCES routers(id),
    end_router_id   UUID REFERENCES routers(id),
    duration_seconds INTEGER,
    amount_paise    INTEGER,
    state           TEXT NOT NULL DEFAULT 'in_progress',
    end_method      TEXT,  -- 'confirmed', 'offline_photo', 'admin_override'
    dispute_flag    BOOLEAN NOT NULL DEFAULT false
);

-- Damage reports
CREATE TABLE reports (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bike_id     UUID NOT NULL REFERENCES bikes(id),
    user_id     UUID REFERENCES users(id),
    report_type TEXT NOT NULL,  -- 'damage', 'vandalism', 'issue'
    description TEXT,
    photo_url   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved    BOOLEAN NOT NULL DEFAULT false
);
```

**TimescaleDB hypertable for telemetry:**

```sql
CREATE TABLE bike_telemetry (
    time        TIMESTAMPTZ NOT NULL,
    bike_id     UUID NOT NULL,
    battery_pct SMALLINT,
    ride_state  TEXT,
    upright     BOOLEAN
);
SELECT create_hypertable('bike_telemetry', 'time');
```

---

## 7. Mobile Application

### 7.1 Framework and Key Dependencies

**Flutter** (Dart). Targets Android 9+ and iOS 14+.

| Package                                       | Purpose                                   |
| --------------------------------------------- | ----------------------------------------- |
| `flutter_background_geolocation` (Transistor) | Background location for campus geofencing |
| `mobile_scanner`                              | QR code scanning                          |
| `firebase_messaging`                          | Push notifications (FCM)                  |
| `dio`                                         | HTTP client                               |
| `riverpod`                                    | State management                          |
| `web_socket_channel`                          | WebSocket for real-time ride state        |
| `google_sign_in`                              | Google OAuth                              |
| `flutter_local_notifications`                 | In-app alerts (balance warnings)          |

### 7.2 Key Screens

- **Home:** Map of campus showing available bikes per parking station, with battery indicators. Grayed-out stations have offline routers.
- **Scan:** Full-screen QR scanner. Validates bike UUID, shows bike status before confirming.
- **Active Ride:** Timer, current balance remaining, elapsed distance (phone GPS), LED state indicator mirrored on screen. Balance warning banner appears at 20% remaining.
- **Ride Summary:** Duration, amount charged, start/end station, map trace.
- **Ride History:** Paginated list of past rides with expandable detail.
- **Wallet:** Current balance, top-up flow, transaction history.
- **Report Issue:** Camera capture + description field, tied to the bike just used.
- **Admin Dashboard** (admin accounts only): Router health grid, active rides count, bikes in orange state.

### 7.3 Geofencing

The app uses the phone's GPS to monitor campus boundaries. This is a soft enforcement mechanism — it triggers a warning notification if the user appears to have left campus with an active ride, but does not force-end the ride. The geofence boundary is a polygon defined in the app's remote config, updatable without an app release.

iOS note: Background location uses "significant location change" mode on iOS, which provides ~500m accuracy. This is sufficient for campus-boundary detection but not for precise station location. Precise location is only required when the app is foregrounded (e.g., during active ride screen).

### 7.4 QR Code Design

The QR code printed on each bicycle encodes a static UUID (`bike_uuid`) only. It is not a credential. The server never acts on a bike UUID alone — it always requires a valid authenticated session alongside the UUID. The UUID is effectively a bicycle's "name tag."

QR codes are printed on tamper-evident stickers. Damaged or missing QR codes are reported via the app.

---

## 8. Security Model

### 8.1 Bicycle Authentication

Each bicycle has a unique **ECDSA key pair** (Curve25519) provisioned at manufacture time. The private key is stored in the ESP32's eFuse-backed secure storage (cannot be read out via UART/JTAG after provisioning). The public key is registered in the server's bike registry.

All commands sent to a bicycle are signed by the server's private key. The bicycle verifies the signature against the server's public key (stored in firmware) before executing any command. Unsigned or invalid-signature frames are silently discarded.

All events sent from a bicycle to the router are signed by the bicycle's private key. The server verifies against the registered public key.

### 8.2 Session Authentication

Users authenticate via **Google OAuth 2.0** (Thapar uses Google Workspace, so all students have `@thapar.edu` accounts). On first login, the user's Google `sub` is linked to their Thapar ID in the database. Subsequent logins are automatic via Google OAuth.

On login, the Hono API issues a **JWT** (RS256, 24-hour expiry) containing `user_id`, `thapar_id`, `role` (`student` or `admin`), and `exp`. The JWT is stored in Flutter's secure storage (`flutter_secure_storage`).

### 8.3 Key Revocation

If a bicycle is tampered with or its key is suspected compromised:

1. Admin marks the bike as `disabled` via the admin API
2. The server stops issuing commands to that bike
3. Router nodes receive a revocation list update and stop relaying commands to that bike's MAC address
4. The bike is physically retrieved and re-provisioned with a new key pair

### 8.4 Replay Attack Prevention

Each server command includes a `server_timestamp` and a `nonce` (random 16 bytes). The bicycle stores the last 10 received nonces in NVS and rejects any command with a duplicate nonce or a timestamp older than 60 seconds.

---

## 9. Billing and Wallet System

### 9.1 Pricing Model

Pricing is configured server-side (not hardcoded). Default: ₹2 per 15 minutes, billed in 15-minute increments. Partial final increments are rounded up.

Amounts are stored in **paise** (integer) throughout the system to avoid floating-point rounding errors.

### 9.2 Balance Sync

At ride start, the user's current wallet balance is sent to the bicycle as part of the unlock command payload. The bicycle counts down locally using its RTC. This means billing continues correctly even if the phone dies or goes offline.

Warning thresholds (beep + notification):

- 30 minutes of balance remaining: single beep every 5 minutes
- 15 minutes remaining: double beep every 2 minutes
- 5 minutes remaining: rapid beep every 30 seconds

At zero balance, the bicycle continues to beep but **does not force-lock**. The ride continues and the wallet goes into deficit. This prevents dangerous situations (e.g., user mid-road when balance hits zero).

### 9.3 Deficit Policy

A wallet in deficit suspends the account. The user sees a "Pay dues" screen on app open and cannot start a new ride. Dues are settled by topping up the wallet to at least ₹0 (dues are cleared, not waived).

At semester rollover, the system generates a dues report. Students with outstanding dues have a flag set in the database that the academic administration can query. This mirrors the library dues enforcement model already in place at Thapar.

### 9.4 Disputed Rides

If a ride ends via the offline photo flow (see Section 10.4) or if the bike syncs a conflicting log, a `dispute_flag` is set on the ride record. Disputed rides appear in the admin dashboard for manual review. The user is billed based on the server's last-known timestamp until the dispute is resolved.

---

## 10. Fault Tolerance and Edge Cases

### 10.1 Router Node Goes Offline

Router nodes send a heartbeat to the server every 30 seconds. The server's fleet model marks a router as `offline` if no heartbeat is received for 90 seconds (3 missed intervals). The mobile app's station map shows offline stations grayed out with a "temporarily unavailable" label.

If a router goes offline while a bike is in orange state (locked, awaiting confirmation), the ride timer continues until the bike can reach any router and send the lock confirmation. The server waits up to 4 hours before auto-resolving the ride as `offline_ended` based on the bike's last-known locked state.

### 10.2 Bicycle Battery Dies During a Ride

The latch mechanism is designed to lock mechanically at any time (user rotates the latch). The electromagnet is only required to **unlock**, not to maintain the locked state. Therefore, if the battery dies mid-ride, the user can still physically lock the bicycle.

If the battery dies before the lock confirmation reaches the server, the user submits a **geotagged photo** of the locked bicycle via the app. The photo is uploaded with the user's GPS coordinates and timestamp. An admin reviews the submission and manually ends the ride at the photo submission timestamp. The bike's NVS log is synced when it next comes online and used to cross-reference.

### 10.3 Phone Dies During a Ride

The bicycle maintains the ride state independently. The timer runs on the bicycle's RTC. When the phone comes back online, the app reconnects to the WebSocket and receives the current ride state. No data is lost.

The bike continues to warn the user via beeper even without phone connectivity.

### 10.4 Improper Parking

Each router node is positioned at a designated bicycle parking stand. The onboard accelerometer is used to determine whether the bicycle is upright (`upright` flag in heartbeat). A bicycle that is locked but not upright triggers an "improperly parked" flag on the server, and the user receives a notification to stand the bicycle correctly before their ride is confirmed ended.

Failure to correct improper parking within 5 minutes results in the ride being ended anyway (the server cannot enforce physical behavior), but the event is logged and repeat offenders can be flagged by an admin.

### 10.5 Two Users Scanning the Same Bike

Bike state transitions on the server are protected by an **atomic reservation lock**. When the server receives a `rides/start` request, it uses a Redis `SET NX` operation on the key `bike_lock:{bike_id}` with a 30-second TTL. Only one request can hold this lock at a time. The second request receives a `409 Conflict` response and the app shows "This bike was just taken — try another."

### 10.6 Student Graduation / Account Deactivation

Student accounts are deactivated at the end of each academic year unless renewed. Deactivation is triggered by a nightly job that queries the university's student directory (LDAP or provided CSV export). Deactivated accounts with zero balance are archived. Deactivated accounts with outstanding dues remain active in the dues registry.

---

## 11. Observability and Operations

### 11.1 Stack

- **Prometheus** — metrics collection from both backend services and router nodes
- **Grafana** — dashboards and alerting
- **Loki** — log aggregation
- **Promtail** — log shipping from services and routers (routers ship logs via MQTT; the Go service forwards to Loki)

### 11.2 Key Metrics

**Per bicycle (via Prometheus, sourced from MQTT telemetry):**

```
unicycle_bike_battery_pct{bike_id}
unicycle_bike_ride_state{bike_id, state}
unicycle_bike_last_seen_seconds{bike_id}        # seconds since last heartbeat
unicycle_bike_lock_actuations_total{bike_id}
unicycle_bike_failed_unlock_total{bike_id}
```

**Per router:**

```
unicycle_router_online{router_id}
unicycle_router_connected_bikes{router_id}
unicycle_router_last_heartbeat_age_seconds{router_id}
unicycle_router_relay_latency_ms{router_id}     # histogram
```

**System-wide:**

```
unicycle_active_rides_total
unicycle_orange_state_duration_seconds          # histogram — key health signal
unicycle_rides_ended_offline_total
unicycle_wallet_deficit_users_total
unicycle_bikes_below_battery_threshold_total
```

`unicycle_orange_state_duration_seconds` is the most important operational signal. A spike in this histogram indicates router coverage problems before heartbeat timeouts surface them.

### 11.3 Alerts

| Alert            | Condition                                | Severity |
| ---------------- | ---------------------------------------- | -------- |
| RouterDown       | `router_last_heartbeat_age > 90s`        | Warning  |
| RouterDownLong   | `router_last_heartbeat_age > 300s`       | Critical |
| BikeNotSeen      | `bike_last_seen > 3600s` during daylight | Warning  |
| OrangeStateSpike | `p95(orange_state_duration) > 120s`      | Warning  |
| HighDeficitUsers | `deficit_users > 10`                     | Info     |
| BikeBatteryFleet | `avg(bike_battery_pct) < 30%`            | Warning  |

### 11.4 Admin Dashboard (Grafana)

- **Fleet map panel:** Campus map with colored dots per bike (green/orange/red/gray)
- **Router health grid:** Table of all routers with online status and connected bike count
- **Active rides panel:** Count and list of in-progress rides
- **Orange state histogram:** Distribution of lock-to-confirm latency over time
- **Battery distribution:** Histogram of fleet battery percentages
- **Damage reports queue:** Unresolved reports with photo previews

---

## 12. Deployment

### 12.1 Infrastructure

A single **Hetzner CX22** VPS (2 vCPU, 4GB RAM, 40GB SSD, ~€4/month) runs the entire backend for campus-scale pilot (up to ~200 bikes, ~50 concurrent rides).

**Docker Compose services:**

```yaml
services:
  unicycle-rt: # Go real-time service
  unicycle-api: # Hono API service
  postgres: # PostgreSQL + TimescaleDB
  redis: # Redis pub/sub
  mosquitto: # MQTT broker
  prometheus: # Metrics
  grafana: # Dashboards
  loki: # Logs
  promtail: # Log shipping
  caddy: # Reverse proxy + automatic TLS
```

### 12.2 Caddy Configuration

Caddy handles TLS automatically via Let's Encrypt. Routes:

```
api.unicycle.thapar.edu    → unicycle-api:3000
ws.unicycle.thapar.edu     → unicycle-rt:8080  (WebSocket)
mqtt.unicycle.thapar.edu   → mosquitto:8883    (MQTT over TLS)
grafana.unicycle.thapar.edu → grafana:3000     (admin only, IP restricted)
```

### 12.3 CI/CD

GitHub Actions pipeline:

1. On push to `main`: run tests, build Docker images, push to GitHub Container Registry
2. Deploy via SSH: pull new images, `docker compose up -d`
3. Health check: hit `/health` endpoint on both services post-deploy

### 12.4 Backup

- PostgreSQL: daily `pg_dump` to Hetzner Object Storage, 30-day retention
- TimescaleDB telemetry: weekly compressed backup, 90-day retention
- Mosquitto persistent session store: daily backup

### 12.5 Scaling Path

The current Compose setup handles a campus pilot comfortably. If Unicycle expands to multiple campuses or significantly more bikes:

1. Promote `postgres` to a managed database (Hetzner Managed PostgreSQL)
2. Replace Redis pub/sub with **NATS JetStream** for durability and multi-region support
3. Separate `unicycle-rt` and `unicycle-api` to dedicated VMs
4. Move to Kubernetes (k3s) on Hetzner with the same Docker images — the Compose → k3s migration is straightforward

---

## 13. Ride State Machine

### States

| State              | LED              | Description                                           |
| ------------------ | ---------------- | ----------------------------------------------------- |
| `available`        | Off              | Bicycle ready to rent, battery above threshold        |
| `battery_disabled` | Slow red blink   | Battery below 15%; cannot be rented                   |
| `ride_requested`   | Off              | Server has received unlock request; reservation held  |
| `unlocking`        | Flickering green | Command delivered; bicycle verifying and actuating    |
| `in_use`           | Green            | Ride active; timer running                            |
| `locking`          | Orange           | Latch rotated; bicycle seeking router                 |
| `lock_unconfirmed` | Orange (steady)  | Router not reachable; timer still running             |
| `ended`            | Red              | Ride confirmed ended by router; billing stopped       |
| `offline_ended`    | Red              | Ride ended via photo submission; pending admin review |
| `unlock_failed`    | Rapid red blink  | Unlock command failed or timed out                    |

### Transitions

```
available ──(QR scanned)──────────────────────→ ride_requested
available ──(battery drops below threshold)───→ battery_disabled
battery_disabled ──(battery recovers)─────────→ available

ride_requested ──(router relays command)──────→ unlocking
ride_requested ──(timeout 30s / no router)────→ unlock_failed
unlock_failed ──(reservation released)────────→ available

unlocking ──(bike confirms unlock)────────────→ in_use
unlocking ──(sig invalid / timeout 10s)───────→ unlock_failed

in_use ──(latch rotated)──────────────────────→ locking

locking ──(router confirms within 30s)────────→ ended
locking ──(no router found within 30s)────────→ lock_unconfirmed

lock_unconfirmed ──(router recovers)──────────→ ended
lock_unconfirmed ──(battery dies / long timeout)→ offline_ended

ended ──(2 min cooldown)──────────────────────→ available
offline_ended ──(admin confirms)──────────────→ available
```

---

## 14. FAQ

**Q: What if I lock the bicycle and the LED stays orange for a long time?**
The bicycle is locked and physically secured, but it hasn't been able to communicate with a router node yet. Your ride timer is still running. Try moving the bicycle slightly closer to the parking station router (within 50–100m). If the router at that station is offline, it will appear grayed out on the app map — in that case, you can try unlocking and riding to the nearest functioning station. If you genuinely cannot reach any router, submit a geotagged photo of the locked bicycle through the app and an admin will manually end your ride.

**Q: What if my phone dies during a ride?**
Nothing bad happens. The bicycle is running its own timer independently. When your phone comes back online and you open the app, it will reconnect and show your current ride status. Lock the bicycle normally and wait for the red LED — your ride has ended correctly regardless of your phone's state.

**Q: What if the bicycle's battery dies before I finish my ride?**
The latch can always be rotated to lock position manually — no electricity is required to lock. Rotate the latch to lock the bicycle, then submit a geotagged photo of the locked bicycle through the app. An admin will end your ride at the time of your photo submission. You will not be billed for time after your submission.

**Q: Can someone steal my bicycle mid-ride?**
The bicycle is physically unlocked during your ride — it's a bicycle, not a vault. However, it cannot be rented by anyone else because the server marks it as `in_use`. A stolen bicycle during an active ride is a physical security matter and should be reported to Thapar campus security. Your ride timer continues until the bicycle is locked and confirmed; campus security can issue an admin override once the incident is verified.

**Q: My wallet went negative. How do I pay?**
Open the app and you will see a "Pay dues" screen. Top up your wallet to bring it to ₹0 or above. Once your balance is non-negative, your account is automatically unsuspended. Outstanding dues at the end of the academic year are subject to the same enforcement as library dues.

**Q: I reported damage on a bicycle but it hasn't been fixed.**
Damage reports go to the operations team's admin dashboard. Fixing a bicycle requires physical intervention by campus maintenance. The app cannot guarantee a response timeline — contact the Unicycle operations team directly if a reported issue is urgent or unaddressed.

**Q: Why do I need to give the app background location permission?**
Background location is used for campus geofencing — the app will warn you if you appear to have ridden outside campus boundaries during an active ride. It is not used to track your location persistently. Location data is not stored on the server; only the GPS coordinates at ride start and ride end (used for billing confirmation) are logged. You can deny background location permission; the geofencing warning will not function but the ride itself will work normally.

**Q: Can two people share a ride?**
No. Each ride is tied to one Thapar ID and one active session. Sharing credentials violates the terms of service and defeats the accountability model. If a non-Thapar person needs transport, a student may accompany them on a separate bicycle.

**Q: What happens to my account when I graduate?**
Accounts are deactivated at the end of the academic year. If you have a zero balance at deactivation, your account is archived. If you have outstanding dues, your account remains in the dues registry until cleared. You will not be able to use the app after deactivation regardless of balance.

**Q: How is the ₹2/15 minutes rate set? Can it change?**
Pricing is configured on the server and can be updated by an administrator without an app update. Changes will be visible in the app's "How pricing works" screen before you start a ride. Price changes do not affect in-progress rides.

**Q: What if two people scan the same bike at the same time?**
The server uses an atomic lock on each bicycle's state. Only one unlock request can be processed at a time. The second person's app will immediately show "This bike was just taken — try another." There is no race condition; the reservation is atomic.

**Q: Is it safe to leave the bicycle in orange state and walk away?**
The bicycle is physically locked and secured. However, your ride timer is running and you are being billed. Do not walk away from an orange-state bicycle unless you are confident you will return shortly or you plan to submit a photo for manual ride-end. Orange state means the lock is confirmed locally on the bicycle but has not been verified by the server yet.

---

_Unicycle v1.0 — Internal specification. Subject to revision._
_Thapar Institute of Engineering & Technology_
