package models

import (
	"time"

	"github.com/google/uuid"
	api "github.com/sauryagur/unicycle/api"
	"gorm.io/gorm"
)

//
// ENUMS
//

// BicycleRideState matches the OpenAPI BicycleRideState enum
type BicycleRideState string

const (
	BicycleStateAvailable       BicycleRideState = "available"
	BicycleStateBatteryDisabled BicycleRideState = "battery_disabled"
	BicycleStateRideRequested   BicycleRideState = "ride_requested"
	BicycleStateUnlocking       BicycleRideState = "unlocking"
	BicycleStateInUse           BicycleRideState = "in_use"
	BicycleStateLocking         BicycleRideState = "locking"
	BicycleStateLockUnconfirmed BicycleRideState = "lock_unconfirmed"
	BicycleStateEnded           BicycleRideState = "ended"
	BicycleStateOfflineEnded    BicycleRideState = "offline_ended"
	BicycleStateUnlockFailed    BicycleRideState = "unlock_failed"
)

// RideState matches the OpenAPI Ride state enum
type RideState string

const (
	RideStateInProgress   RideState = "in_progress"
	RideStateEnded        RideState = "ended"
	RideStateOfflineEnded RideState = "offline_ended"
	// RideStateCancelled Note: Cancelled is not in the OpenAPI spec, but you may want to keep it
	// for internal use. If you expose it via API, you'll need to add it to the spec.
	RideStateCancelled RideState = "cancelled"
)

type RideEndMethod string

const (
	RideEndMethodConfirmed     RideEndMethod = "confirmed"
	RideEndMethodOfflinePhoto  RideEndMethod = "offline_photo"
	RideEndMethodAdminOverride RideEndMethod = "admin_override"
)

type UserRole string

const (
	UserRoleStudent UserRole = "student"
	UserRoleAdmin   UserRole = "admin"
)

type TransactionType string

const (
	TransactionTypeRideCharge TransactionType = "ride_charge"
	TransactionTypeTopup      TransactionType = "topup"
	TransactionTypeRefund     TransactionType = "refund"
	TransactionTypeAdjustment TransactionType = "adjustment"
)

type ReportType string

const (
	ReportTypeDamage          ReportType = "damage"
	ReportTypeVandalism       ReportType = "vandalism"
	ReportTypeMalfunction     ReportType = "malfunction"
	ReportTypeImproperParking ReportType = "improper_parking"
)

//
// BASE
//

type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time      `gorm:"not null;default:now()" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

//
// USER
//

type User struct {
	BaseModel

	ThaparID    string   `gorm:"type:text;uniqueIndex;not null"`
	Email       string   `gorm:"type:text;uniqueIndex;not null"`
	GoogleSub   string   `gorm:"type:text;uniqueIndex;not null"`
	WalletPaise int      `gorm:"not null;default:0"`
	Suspended   bool     `gorm:"not null;default:false"`
	Role        UserRole `gorm:"type:text;not null;default:'student'"`
	Name        string   `gorm:"type:text"`

	Rides        []Ride        `gorm:"foreignKey:UserID;references:ID"`
	Reports      []Report      `gorm:"foreignKey:UserID;references:ID"`
	Transactions []Transaction `gorm:"foreignKey:UserID;references:ID"`
}

func (User) TableName() string { return "users" }

//
// BICYCLE
//

type Bicycle struct {
	BaseModel

	MACAddress    string           `gorm:"type:macaddr;uniqueIndex;not null"`
	SerialNumber  string           `gorm:"type:text;uniqueIndex;not null"`
	PublicKey     []byte           `gorm:"type:bytea;not null"`
	State         BicycleRideState `gorm:"type:text;not null;default:'available'"`
	BatteryPct    *int             `gorm:"type:integer"`
	LastSeenAt    *time.Time       `gorm:"type:timestamptz"`
	Disabled      bool             `gorm:"not null;default:false"`
	CurrentRideID *uuid.UUID       `gorm:"type:uuid;index"`

	Rides   []Ride   `gorm:"foreignKey:BikeID;references:ID"`
	Reports []Report `gorm:"foreignKey:BikeID;references:ID"`
}

func (Bicycle) TableName() string { return "bicycles" }

//
// ROUTER
//

type Router struct {
	BaseModel

	LocationName string     `gorm:"type:text;not null"`
	LastSeenAt   *time.Time `gorm:"type:timestamptz"`
	Online       bool       `gorm:"not null;default:false"`

	StartRides []Ride `gorm:"foreignKey:StartRouterID;references:ID"`
	EndRides   []Ride `gorm:"foreignKey:EndRouterID;references:ID"`
}

func (Router) TableName() string { return "routers" }

//
// RIDE
//

type Ride struct {
	BaseModel

	UserID uuid.UUID `gorm:"type:uuid;not null;index:idx_rides_user_started,priority:1"`
	BikeID uuid.UUID `gorm:"type:uuid;not null;index:idx_rides_bike"`

	StartedAt time.Time  `gorm:"type:timestamptz;not null;index:idx_rides_user_started,priority:2"`
	EndedAt   *time.Time `gorm:"type:timestamptz"`

	DurationSeconds *int
	AmountPaise     *int

	State RideState `gorm:"type:text;not null;default:'in_progress';index"`

	StartRouterID *uuid.UUID `gorm:"type:uuid;index"`
	EndRouterID   *uuid.UUID `gorm:"type:uuid;index"`

	EndMethod   *RideEndMethod `gorm:"type:text"`
	DisputeFlag bool           `gorm:"not null;default:false"`

	User User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
	Bike Bicycle `gorm:"foreignKey:BikeID;references:ID;constraint:OnDelete:CASCADE"`

	StartRouter *Router `gorm:"foreignKey:StartRouterID;references:ID;constraint:OnDelete:SET NULL"`
	EndRouter   *Router `gorm:"foreignKey:EndRouterID;references:ID;constraint:OnDelete:SET NULL"`
}

func (Ride) TableName() string { return "rides" }

//
// REPORT
//

type Report struct {
	BaseModel

	BikeID     uuid.UUID  `gorm:"type:uuid;not null;index"`
	UserID     *uuid.UUID `gorm:"type:uuid;index"`
	ReportType ReportType `gorm:"type:text;not null;index"`

	Description     *string `gorm:"type:text"`
	PhotoURL        *string `gorm:"type:text"`
	Resolved        bool    `gorm:"not null;default:false;index"`
	ResolutionNotes *string `gorm:"type:text"`

	Bike Bicycle `gorm:"foreignKey:BikeID;references:ID;constraint:OnDelete:CASCADE"`
	User *User   `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:SET NULL"`
}

func (Report) TableName() string { return "reports" }

//
// TRANSACTION
//

type Transaction struct {
	BaseModel

	UserID uuid.UUID `gorm:"type:uuid;not null;index"`

	AmountPaise       int             `gorm:"not null"`
	Type              TransactionType `gorm:"type:text;not null;index"`
	Description       string          `gorm:"type:text"`
	BalanceAfterPaise int             `gorm:"not null"`

	User User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (Transaction) TableName() string { return "transactions" }

//
// TELEMETRY
//

type BikeTelemetry struct {
	BikeID     uuid.UUID        `gorm:"type:uuid;not null;index:idx_telemetry_bike_time,priority:1"`
	Time       time.Time        `gorm:"type:timestamptz;not null;index:idx_telemetry_bike_time,priority:2"`
	BatteryPct *int16           `gorm:"type:smallint"`
	State      BicycleRideState `gorm:"type:text"`
	Upright    *bool

	Bike Bicycle `gorm:"foreignKey:BikeID;references:ID;constraint:OnDelete:CASCADE"`
}

func (BikeTelemetry) TableName() string { return "bike_telemetry" }

// Helper function to convert between GORM state and API state
func (b *Bicycle) ToAPIBicycle() api.Bicycle {
	return api.Bicycle{
		Id:            b.ID,
		SerialNumber:  b.SerialNumber,
		MacAddress:    b.MACAddress,
		State:         api.BicycleRideState(b.State),
		BatteryPct:    b.BatteryPct,
		LastSeenAt:    b.LastSeenAt,
		CurrentRideId: b.CurrentRideID,
		Disabled:      b.Disabled,
	}
}

// Helper function to convert API state to GORM state
func (b *Bicycle) FromAPIBicycle(apiBike api.Bicycle) {
	b.State = BicycleRideState(apiBike.State)
	b.MACAddress = apiBike.MacAddress
	b.SerialNumber = apiBike.SerialNumber
	b.Disabled = apiBike.Disabled
	b.BatteryPct = apiBike.BatteryPct
	b.LastSeenAt = apiBike.LastSeenAt
	b.CurrentRideID = apiBike.CurrentRideId
}
