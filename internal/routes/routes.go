package routes

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appauth "github.com/sauryagur/unicycle/internal/auth"
	"github.com/sauryagur/unicycle/internal/integrations"
	"github.com/sauryagur/unicycle/internal/models"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Store is deliberately small so handlers are independently testable. The
// production application can replace this state with repository/service wiring.
type Store struct {
	sync.RWMutex
	Users        map[uuid.UUID]*models.User
	Tokens       map[string]uuid.UUID
	Bikes        map[uuid.UUID]*models.Bicycle
	Rides        map[uuid.UUID]*models.Ride
	Reports      map[uuid.UUID]*models.Report
	Transactions map[uuid.UUID]*models.Transaction
	DB           *gorm.DB
	JWT          *appauth.JWT
	Google       *integrations.GoogleOAuth
	Redis        *integrations.Redis
	MQTT         *integrations.Publisher
}

func NewStore() *Store {
	return &Store{Users: map[uuid.UUID]*models.User{}, Tokens: map[string]uuid.UUID{}, Bikes: map[uuid.UUID]*models.Bicycle{}, Rides: map[uuid.UUID]*models.Ride{}, Reports: map[uuid.UUID]*models.Report{}, Transactions: map[uuid.UUID]*models.Transaction{}}
}

func NewProductionStore(db *gorm.DB, signer *appauth.JWT, google *integrations.GoogleOAuth, redis *integrations.Redis, mqtt *integrations.Publisher) *Store {
	s := NewStore()
	s.DB, s.JWT, s.Google, s.Redis, s.MQTT = db, signer, google, redis, mqtt
	if db != nil {
		var users []models.User
		if db.Find(&users).Error == nil {
			for i := range users {
				s.Users[users[i].ID] = &users[i]
			}
		}
		var bikes []models.Bicycle
		if db.Find(&bikes).Error == nil {
			for i := range bikes {
				s.Bikes[bikes[i].ID] = &bikes[i]
			}
		}
		var rides []models.Ride
		if db.Find(&rides).Error == nil {
			for i := range rides {
				s.Rides[rides[i].ID] = &rides[i]
			}
		}
		var reports []models.Report
		if db.Find(&reports).Error == nil {
			for i := range reports {
				s.Reports[reports[i].ID] = &reports[i]
			}
		}
		var transactions []models.Transaction
		if db.Find(&transactions).Error == nil {
			for i := range transactions {
				s.Transactions[transactions[i].ID] = &transactions[i]
			}
		}
	}
	return s
}

func (s *Store) reload() {
	if s.DB == nil {
		return
	}
	var users []models.User
	var bikes []models.Bicycle
	var rides []models.Ride
	var reports []models.Report
	var transactions []models.Transaction
	if s.DB.Find(&users).Error != nil || s.DB.Find(&bikes).Error != nil || s.DB.Find(&rides).Error != nil || s.DB.Find(&reports).Error != nil || s.DB.Find(&transactions).Error != nil {
		return
	}
	s.Lock()
	defer s.Unlock()
	for i := range users {
		s.Users[users[i].ID] = &users[i]
	}
	for i := range bikes {
		s.Bikes[bikes[i].ID] = &bikes[i]
	}
	for i := range rides {
		s.Rides[rides[i].ID] = &rides[i]
	}
	for i := range reports {
		s.Reports[reports[i].ID] = &reports[i]
	}
	for i := range transactions {
		s.Transactions[transactions[i].ID] = &transactions[i]
	}
}

var defaultStore = NewStore()

func SetupRoutes(r *gin.Engine) { SetupRoutesWithStore(r, defaultStore) }
func SetupRoutesWithStore(r *gin.Engine, s *Store) {
	v := r.Group("/v1")
	v.GET("/health", func(c *gin.Context) { health(c, s) })
	a := v.Group("/auth")
	a.POST("/google", func(c *gin.Context) { google(c, s) })
	a.POST("/refresh", auth(s), func(c *gin.Context) {
		token := uuid.NewString()
		if s.JWT != nil {
			s.RLock()
			u := s.Users[uid(c)]
			s.RUnlock()
			if u == nil {
				fail(c, 401, "UNAUTHORIZED", "User not found")
				return
			}
			var err error
			token, err = s.JWT.Issue(u.ID, u.ThaparID, string(u.Role))
			if err != nil {
				fail(c, 500, "INTERNAL_ERROR", "Unable to issue token")
				return
			}
		}
		s.Lock()
		if s.JWT == nil {
			s.Tokens[token] = uid(c)
		}
		s.Unlock()
		c.JSON(200, gin.H{"token": token})
	})
	a.POST("/logout", auth(s), func(c *gin.Context) {
		p := strings.Fields(c.GetHeader("Authorization"))
		if s.JWT != nil && s.Redis != nil {
			if claims, err := s.JWT.Parse(p[1]); err == nil && claims.ID != "" {
				_ = s.Redis.Revoke(c, "jwt:revoked:"+claims.ID, 24*time.Hour)
			}
		}
		s.Lock()
		delete(s.Tokens, p[1])
		s.Unlock()
		c.Status(204)
	})
	a.GET("/me", auth(s), func(c *gin.Context) { me(c, s) })
	p := v.Group("", auth(s))
	p.GET("/bikes", func(c *gin.Context) { bikes(c, s) })
	p.GET("/bikes/:bike_id", func(c *gin.Context) { bike(c, s) })
	p.GET("/bikes/:bike_id/status", func(c *gin.Context) { status(c, s) })
	p.POST("/rides/start", func(c *gin.Context) { start(c, s) })
	p.GET("/rides/current", func(c *gin.Context) { current(c, s) })
	p.GET("/rides/history", func(c *gin.Context) { history(c, s) })
	p.GET("/rides/:ride_id", func(c *gin.Context) { ride(c, s) })
	p.POST("/rides/:ride_id/end", func(c *gin.Context) { end(c, s) })
	p.GET("/wallet/balance", func(c *gin.Context) { balance(c, s) })
	p.POST("/wallet/topup", func(c *gin.Context) { topup(c, s) })
	p.GET("/wallet/transactions", func(c *gin.Context) { txns(c, s) })
	p.POST("/reports", func(c *gin.Context) { reportCreate(c, s) })
	p.GET("/reports/:report_id", func(c *gin.Context) { report(c, s) })
	ad := v.Group("/admin", auth(s))
	ad.GET("/fleet", admin(s), func(c *gin.Context) { fleet(c, s) })
	ad.GET("/routers", admin(s), func(c *gin.Context) { c.JSON(200, gin.H{"routers": []any{}}) })
	ad.POST("/bikes/:bike_id/disable", admin(s), func(c *gin.Context) { toggle(c, s, true) })
	ad.POST("/bikes/:bike_id/enable", admin(s), func(c *gin.Context) { toggle(c, s, false) })
	ad.GET("/reports", admin(s), func(c *gin.Context) { reports(c, s) })
	ad.POST("/reports/:report_id/resolve", admin(s), func(c *gin.Context) { resolve(c, s) })
}
func fail(c *gin.Context, n int, code, msg string) {
	c.AbortWithStatusJSON(n, gin.H{"code": code, "message": msg})
}
func health(c *gin.Context, s *Store) {
	checks := gin.H{"database": "ok", "redis": "ok", "mqtt": "ok"}
	unhealthy := false
	if s.DB != nil {
		if db, err := s.DB.DB(); err != nil || db.PingContext(c) != nil {
			checks["database"] = "error"
			unhealthy = true
		}
	}
	if s.Redis != nil {
		if s.Redis.Ping(c) != nil {
			checks["redis"] = "error"
			unhealthy = true
		}
	}
	if s.MQTT != nil && !s.MQTT.Client.IsConnected() {
		checks["mqtt"] = "error"
		unhealthy = true
	}
	status := "healthy"
	code := 200
	if unhealthy {
		status, code = "unhealthy", 503
	}
	c.JSON(code, gin.H{"status": status, "version": "1.0.0", "uptime": "0s", "checks": checks})
}
func parseID(c *gin.Context, k string) (uuid.UUID, bool) {
	x, e := uuid.Parse(c.Param(k))
	if e != nil {
		fail(c, 400, "VALIDATION_ERROR", "invalid "+k)
		return uuid.Nil, false
	}
	return x, true
}
func auth(s *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		s.reload()
		p := strings.Fields(c.GetHeader("Authorization"))
		if len(p) != 2 || !strings.EqualFold(p[0], "Bearer") {
			fail(c, 401, "UNAUTHORIZED", "Missing or invalid authentication token")
			return
		}
		if s.JWT != nil {
			claims, err := s.JWT.Parse(p[1])
			if err != nil {
				fail(c, 401, "UNAUTHORIZED", "Missing or invalid authentication token")
				return
			}
			if s.Redis != nil && claims.ID != "" {
				revoked, err := s.Redis.Revoked(c, "jwt:revoked:"+claims.ID)
				if err != nil {
					fail(c, 503, "SERVICE_UNAVAILABLE", "Authentication service unavailable")
					return
				}
				if revoked {
					fail(c, 401, "UNAUTHORIZED", "Token has been revoked")
					return
				}
			}
			c.Set("uid", claims.UserID)
			c.Next()
			return
		}
		s.RLock()
		u, ok := s.Tokens[p[1]]
		s.RUnlock()
		if !ok {
			fail(c, 401, "UNAUTHORIZED", "Missing or invalid authentication token")
			return
		}
		c.Set("uid", u)
		c.Next()
	}
}
func admin(s *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		s.RLock()
		u := s.Users[c.MustGet("uid").(uuid.UUID)]
		s.RUnlock()
		if u == nil || u.Role != models.UserRoleAdmin {
			fail(c, 403, "FORBIDDEN", "Admin privileges required")
			return
		}
		c.Next()
	}
}
func uid(c *gin.Context) uuid.UUID { return c.MustGet("uid").(uuid.UUID) }
func google(c *gin.Context, s *Store) {
	var in struct {
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri"`
	}
	if c.ShouldBindJSON(&in) != nil || strings.TrimSpace(in.Code) == "" {
		fail(c, 400, "VALIDATION_ERROR", "code is required")
		return
	}
	var u *models.User
	if s.Google != nil {
		gu, err := s.Google.Exchange(context.Background(), in.Code, in.RedirectURI)
		if err != nil {
			fail(c, 401, "UNAUTHORIZED", "Google authentication failed")
			return
		}
		u = &models.User{BaseModel: models.BaseModel{ID: uuid.New(), CreatedAt: time.Now().UTC()}, ThaparID: gu.Email, Email: gu.Email, GoogleSub: gu.Sub, Name: gu.Name, Role: models.UserRoleStudent}
	} else {
		u = &models.User{BaseModel: models.BaseModel{ID: uuid.New(), CreatedAt: time.Now().UTC()}, ThaparID: in.Code, Email: in.Code + "@thapar.edu", GoogleSub: in.Code, Role: models.UserRoleStudent}
	}
	t := uuid.NewString()
	if s.JWT != nil {
		var err error
		t, err = s.JWT.Issue(u.ID, u.ThaparID, string(u.Role))
		if err != nil {
			fail(c, 500, "INTERNAL_ERROR", "Unable to issue token")
			return
		}
	}
	s.Lock()
	s.Users[u.ID] = u
	if s.JWT == nil {
		s.Tokens[t] = u.ID
	}
	if s.DB != nil {
		if err := s.DB.Where("google_sub = ?", u.GoogleSub).FirstOrCreate(u).Error; err != nil {
			s.Unlock()
			fail(c, 500, "INTERNAL_ERROR", "Unable to persist user")
			return
		}
	}
	s.Unlock()
	c.JSON(200, gin.H{"token": t, "user": userOut(u)})
}
func userOut(u *models.User) gin.H {
	return gin.H{"id": u.ID, "thapar_id": u.ThaparID, "email": u.Email, "name": u.Name, "role": u.Role, "suspended": u.Suspended, "wallet_balance_paise": u.WalletPaise, "created_at": u.CreatedAt}
}
func me(c *gin.Context, s *Store) {
	s.RLock()
	u := s.Users[uid(c)]
	s.RUnlock()
	if u == nil {
		fail(c, 401, "UNAUTHORIZED", "User not found")
		return
	}
	c.JSON(200, userOut(u))
}
func page(c *gin.Context, d int) (int, int, bool) {
	l, o := d, 0
	var e error
	if c.Query("limit") != "" {
		l, e = strconv.Atoi(c.Query("limit"))
	}
	if e != nil || l < 1 || l > 100 {
		fail(c, 400, "VALIDATION_ERROR", "invalid limit")
		return 0, 0, false
	}
	if c.Query("offset") != "" {
		o, e = strconv.Atoi(c.Query("offset"))
	}
	if e != nil || o < 0 {
		fail(c, 400, "VALIDATION_ERROR", "invalid offset")
		return 0, 0, false
	}
	return l, o, true
}
func bikeOut(b *models.Bicycle) gin.H {
	return gin.H{"id": b.ID, "serial_number": b.SerialNumber, "mac_address": b.MACAddress, "state": b.State, "battery_pct": b.BatteryPct, "last_seen_at": b.LastSeenAt, "current_ride_id": b.CurrentRideID, "disabled": b.Disabled}
}
func bike(c *gin.Context, s *Store) {
	x, ok := parseID(c, "bike_id")
	if !ok {
		return
	}
	s.RLock()
	b := s.Bikes[x]
	s.RUnlock()
	if b == nil {
		fail(c, 404, "NOT_FOUND", "Bicycle not found")
		return
	}
	c.JSON(200, bikeOut(b))
}
func bikes(c *gin.Context, s *Store) {
	l, o, ok := page(c, 50)
	if !ok {
		return
	}
	s.RLock()
	a := []gin.H{}
	for _, b := range s.Bikes {
		if q := c.Query("state"); q != "" && q != string(b.State) {
			continue
		}
		a = append(a, bikeOut(b))
	}
	s.RUnlock()
	n := len(a)
	if o > n {
		o = n
	}
	e := o + l
	if e > n {
		e = n
	}
	c.JSON(200, gin.H{"data": a[o:e], "total": n, "limit": l, "offset": o})
}
func status(c *gin.Context, s *Store) {
	x, ok := parseID(c, "bike_id")
	if !ok {
		return
	}
	s.RLock()
	b := s.Bikes[x]
	s.RUnlock()
	if b == nil {
		fail(c, 404, "NOT_FOUND", "Bicycle not found")
		return
	}
	color := "off"
	if b.State == models.BicycleStateInUse {
		color = "green"
	}
	if b.State == models.BicycleStateLocking || b.State == models.BicycleStateLockUnconfirmed {
		color = "orange"
	}
	c.JSON(200, gin.H{"bike_id": x, "state": b.State, "battery_pct": b.BatteryPct, "last_heartbeat_at": b.LastSeenAt, "is_upright": nil, "router_id": nil, "led_color": color, "battery_warning": "none"})
}
func rideOut(r *models.Ride) gin.H {
	return gin.H{"id": r.ID, "user_id": r.UserID, "bike_id": r.BikeID, "started_at": r.StartedAt, "ended_at": r.EndedAt, "duration_seconds": r.DurationSeconds, "amount_paise": r.AmountPaise, "state": r.State, "start_router_id": r.StartRouterID, "end_router_id": r.EndRouterID, "end_method": r.EndMethod, "dispute_flag": r.DisputeFlag}
}
func start(c *gin.Context, s *Store) {
	var in struct {
		BikeID        uuid.UUID  `json:"bike_id"`
		StartRouterID *uuid.UUID `json:"start_router_id,omitempty"`
	}
	if c.ShouldBindJSON(&in) != nil || in.BikeID == uuid.Nil {
		fail(c, 400, "VALIDATION_ERROR", "bike_id is required")
		return
	}
	s.Lock()
	defer s.Unlock()
	u := s.Users[uid(c)]
	b := s.Bikes[in.BikeID]
	if u == nil {
		fail(c, 401, "UNAUTHORIZED", "User not found")
		return
	}
	if u.Suspended || u.WalletPaise < 1000 {
		fail(c, 402, "INSUFFICIENT_BALANCE", "Wallet balance is insufficient")
		return
	}
	if b == nil || b.Disabled || b.State != models.BicycleStateAvailable {
		fail(c, 409, "BIKE_UNAVAILABLE", "This bike is unavailable")
		return
	}
	if s.Redis != nil {
		reserved, err := s.Redis.Reserve(c, "bike_lock:"+in.BikeID.String(), 30*time.Second)
		if err != nil {
			fail(c, 500, "INTERNAL_ERROR", "Unable to reserve bicycle")
			return
		}
		if !reserved {
			fail(c, 409, "BIKE_UNAVAILABLE", "This bike is already reserved")
			return
		}
	}
	r := &models.Ride{BaseModel: models.BaseModel{ID: uuid.New(), CreatedAt: time.Now().UTC()}, UserID: u.ID, BikeID: b.ID, StartedAt: time.Now().UTC(), State: models.RideStateInProgress, StartRouterID: in.StartRouterID}
	s.Rides[r.ID] = r
	b.State = models.BicycleStateRideRequested
	b.CurrentRideID = &r.ID
	if s.DB != nil {
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(r).Error; err != nil {
				return err
			}
			return tx.Save(b).Error
		}); err != nil {
			fail(c, 500, "INTERNAL_ERROR", "Unable to persist ride")
			return
		}
	}
	if s.MQTT != nil {
		payload, _ := json.Marshal(gin.H{"bike_id": b.ID, "ride_id": r.ID, "user_id": u.ID, "command": "unlock"})
		if err := s.MQTT.Publish("commands/"+b.ID.String(), payload); err != nil {
			fail(c, 500, "INTERNAL_ERROR", "Unable to publish unlock command")
			return
		}
	}
	c.JSON(201, rideOut(r))
}
func ride(c *gin.Context, s *Store) {
	x, ok := parseID(c, "ride_id")
	if !ok {
		return
	}
	s.RLock()
	r := s.Rides[x]
	s.RUnlock()
	if r == nil {
		fail(c, 404, "NOT_FOUND", "Ride not found")
		return
	}
	if r.UserID != uid(c) {
		fail(c, 403, "FORBIDDEN", "Access denied")
		return
	}
	c.JSON(200, rideOut(r))
}
func current(c *gin.Context, s *Store) {
	s.RLock()
	defer s.RUnlock()
	for _, r := range s.Rides {
		if r.UserID == uid(c) && r.State == models.RideStateInProgress {
			c.JSON(200, rideOut(r))
			return
		}
	}
	fail(c, 404, "NO_ACTIVE_RIDE", "You have no ride in progress")
}
func history(c *gin.Context, s *Store) {
	l, o, ok := page(c, 20)
	if !ok {
		return
	}
	s.RLock()
	a := []gin.H{}
	for _, r := range s.Rides {
		if r.UserID == uid(c) {
			a = append(a, rideOut(r))
		}
	}
	s.RUnlock()
	n := len(a)
	if o > n {
		o = n
	}
	e := o + l
	if e > n {
		e = n
	}
	c.JSON(200, gin.H{"data": a[o:e], "total": n, "limit": l, "offset": o})
}
func end(c *gin.Context, s *Store) {
	x, ok := parseID(c, "ride_id")
	if !ok {
		return
	}
	var in struct {
		PhotoURL  string  `json:"photo_url"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if c.ShouldBindJSON(&in) != nil || in.PhotoURL == "" || in.Latitude < -90 || in.Latitude > 90 || in.Longitude < -180 || in.Longitude > 180 {
		fail(c, 400, "VALIDATION_ERROR", "invalid offline ride evidence")
		return
	}
	s.Lock()
	defer s.Unlock()
	r := s.Rides[x]
	if r == nil {
		fail(c, 404, "NOT_FOUND", "Ride not found")
		return
	}
	if r.UserID != uid(c) {
		fail(c, 403, "FORBIDDEN", "Access denied")
		return
	}
	if r.State != models.RideStateInProgress {
		fail(c, 409, "RIDE_NOT_ELIGIBLE", "This ride has already ended")
		return
	}
	m := models.RideEndMethodOfflinePhoto
	r.EndMethod = &m
	r.State = models.RideStateOfflineEnded
	r.DisputeFlag = true
	if s.DB != nil {
		if err := s.DB.Save(r).Error; err != nil {
			fail(c, 500, "INTERNAL_ERROR", "Unable to persist ride")
			return
		}
	}
	c.JSON(200, rideOut(r))
}
func balance(c *gin.Context, s *Store) {
	s.RLock()
	u := s.Users[uid(c)]
	s.RUnlock()
	if u == nil {
		fail(c, 401, "UNAUTHORIZED", "User not found")
		return
	}
	c.JSON(200, gin.H{"balance_paise": u.WalletPaise, "formatted": "₹" + strconv.FormatFloat(float64(u.WalletPaise)/100, 'f', 2, 64), "deficit": u.WalletPaise < 0})
}
func topup(c *gin.Context, s *Store) {
	var in struct {
		Amount  int    `json:"amount_paise"`
		Payment string `json:"payment_method"`
	}
	if c.ShouldBindJSON(&in) != nil || in.Amount < 100 || in.Payment != "" && in.Payment != "campus_wallet" {
		fail(c, 400, "VALIDATION_ERROR", "invalid top-up")
		return
	}
	s.Lock()
	defer s.Unlock()
	u := s.Users[uid(c)]
	old := u.WalletPaise
	u.WalletPaise += in.Amount
	t := &models.Transaction{BaseModel: models.BaseModel{ID: uuid.New()}, UserID: u.ID, AmountPaise: in.Amount, Type: models.TransactionTypeTopup, BalanceAfterPaise: u.WalletPaise}
	s.Transactions[t.ID] = t
	if s.DB != nil {
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(u).Error; err != nil {
				return err
			}
			return tx.Create(t).Error
		}); err != nil {
			fail(c, 500, "INTERNAL_ERROR", "Unable to persist top-up")
			return
		}
	}
	c.JSON(200, gin.H{"transaction_id": t.ID, "new_balance_paise": u.WalletPaise, "old_balance_paise": old, "amount_added_paise": in.Amount})
}
func txns(c *gin.Context, s *Store) {
	l, o, ok := page(c, 20)
	if !ok {
		return
	}
	s.RLock()
	a := []*models.Transaction{}
	for _, t := range s.Transactions {
		if t.UserID == uid(c) {
			a = append(a, t)
		}
	}
	s.RUnlock()
	n := len(a)
	if o > n {
		o = n
	}
	e := o + l
	if e > n {
		e = n
	}
	c.JSON(200, gin.H{"data": a[o:e], "total": n, "limit": l, "offset": o})
}
func reportCreate(c *gin.Context, s *Store) {
	var in struct {
		BikeID      uuid.UUID `json:"bike_id"`
		ReportType  string    `json:"report_type"`
		Description string    `json:"description"`
		PhotoURL    string    `json:"photo_url"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "VALIDATION_ERROR", "invalid report")
		return
	}
	validType := in.ReportType == "damage" || in.ReportType == "vandalism" || in.ReportType == "malfunction" || in.ReportType == "improper_parking"
	if in.BikeID == uuid.Nil || !validType || in.Description != "" && len(in.Description) > 500 {
		fail(c, 400, "VALIDATION_ERROR", "invalid report")
		return
	}
	s.Lock()
	defer s.Unlock()
	if s.Bikes[in.BikeID] == nil {
		fail(c, 400, "VALIDATION_ERROR", "bike not found")
		return
	}
	r := &models.Report{BaseModel: models.BaseModel{ID: uuid.New()}, BikeID: in.BikeID, UserID: &[]uuid.UUID{uid(c)}[0], ReportType: models.ReportType(in.ReportType), Description: &in.Description, PhotoURL: &in.PhotoURL}
	s.Reports[r.ID] = r
	if s.DB != nil {
		if err := s.DB.Create(r).Error; err != nil {
			fail(c, 500, "INTERNAL_ERROR", "Unable to persist report")
			return
		}
	}
	c.JSON(201, r)
}
func report(c *gin.Context, s *Store) {
	x, ok := parseID(c, "report_id")
	if !ok {
		return
	}
	s.RLock()
	r := s.Reports[x]
	s.RUnlock()
	if r == nil {
		fail(c, 404, "NOT_FOUND", "Report not found")
		return
	}
	if r.UserID == nil || *r.UserID != uid(c) {
		s.RLock()
		u := s.Users[uid(c)]
		s.RUnlock()
		if u == nil || u.Role != models.UserRoleAdmin {
			fail(c, 403, "FORBIDDEN", "Access denied")
			return
		}
	}
	c.JSON(200, r)
}
func fleet(c *gin.Context, s *Store) {
	if c == nil {
		return
	}
	s.RLock()
	counts := map[string]int{}
	for _, b := range s.Bikes {
		counts[string(b.State)]++
	}
	n := len(s.Bikes)
	s.RUnlock()
	c.JSON(200, gin.H{"total_bikes": n, "bikes_by_state": counts, "active_rides": 0, "orange_state_count": counts["locking"] + counts["lock_unconfirmed"], "router_count": 0, "online_routers": 0, "fleet_health": "good"})
}
func toggle(c *gin.Context, s *Store, on bool) {
	x, ok := parseID(c, "bike_id")
	if !ok {
		return
	}
	s.Lock()
	b := s.Bikes[x]
	if b != nil {
		b.Disabled = on
		if s.DB != nil {
			_ = s.DB.Save(b).Error
		}
	}
	s.Unlock()
	if b == nil {
		fail(c, 404, "NOT_FOUND", "Bicycle not found")
		return
	}
	c.JSON(200, bikeOut(b))
}
func reports(c *gin.Context, s *Store) {
	l, o, ok := page(c, 20)
	if !ok {
		return
	}
	s.RLock()
	a := []*models.Report{}
	for _, r := range s.Reports {
		a = append(a, r)
	}
	s.RUnlock()
	n := len(a)
	if o > n {
		o = n
	}
	e := o + l
	if e > n {
		e = n
	}
	c.JSON(200, gin.H{"data": a[o:e], "total": n, "limit": l, "offset": o})
}
func resolve(c *gin.Context, s *Store) {
	x, ok := parseID(c, "report_id")
	if !ok {
		return
	}
	s.Lock()
	r := s.Reports[x]
	if r != nil {
		r.Resolved = true
		if s.DB != nil {
			_ = s.DB.Save(r).Error
		}
	}
	s.Unlock()
	if r == nil {
		fail(c, 404, "NOT_FOUND", "Report not found")
		return
	}
	c.JSON(200, r)
}
