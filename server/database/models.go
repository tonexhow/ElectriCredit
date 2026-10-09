package database

// Money is stored as fixed five-decimal peso units.
// Examples:
//
//	5000000   = ₱50.00000
//	-300000   = -₱3.00000
//	12        = ₱0.00012
type Money int64

type User struct {
	ID             int64
	FirstName      string
	LastName       string
	Email          *string
	Phone          *string
	Room           *string
	ProfileImage   *string
	BirthDate      *string
	Gender         *string
	LinksJSON      *string
	DebtLimitUnits *Money
	Status         string
	CreatedAt      int64
	UpdatedAt      int64
}

type Superuser struct {
	ID              int64
	FirstName       string
	LastName        string
	Email           string
	Phone           *string
	ProfileImage    *string
	BirthDate       *string
	LinksJSON       *string
	Rank            string
	PasswordHash    string
	IsHeadDeveloper bool
	Status          string
	CreatedAt       int64
	UpdatedAt       int64
}

type Card struct {
	ID           int64
	UID          string
	UserID       int64
	BalanceUnits Money
	Status       string
	CreatedAt    int64
	UpdatedAt    int64
}

type Hub struct {
	ID         int64
	MACAddress string
	Name       string
	Location   *string
	Status     string
	CreatedAt  int64
	UpdatedAt  int64
}

type Registry struct {
	ID         int64
	MACAddress string
	Name       string
	Location   *string
	Status     string
	CreatedAt  int64
	UpdatedAt  int64
}

type RegistrySession struct {
	ID         int64
	RegistryID int64
	CardID     *int64
	StartedAt  int64
	EndedAt    *int64
	Status     string
}

type PowerSession struct {
	ID        int64
	CardID    int64
	HubID     int64
	StartedAt int64
	EndedAt   *int64
	EnergyWh  int64
	CostUnits Money
	Status    string
}

type PriceConfiguration struct {
	ID              int64
	BaseRateUnits   Money
	TenantRateUnits Money
	DebtLimitUnits  Money
	ChangeNote      *string
	UpdatedBy       *int64
	CreatedAt       int64
	UpdatedAt       int64
}

type Announcement struct {
	ID        int64
	Title     string
	Message   string
	CreatedBy *int64
	CreatedAt int64
	UpdatedAt int64
}

type Log struct {
	ID         int64
	ActorID    *int64
	Level      string
	Action     string
	EntityType *string
	EntityID   *int64
	Message    string
	CreatedAt  int64
}

type LoginSession struct {
	ID          int64
	SuperuserID int64
	TokenHash   string
	ExpiresAt   int64
	CreatedAt   int64
	LastSeenAt  int64
}

type PasswordRecoveryOTP struct {
	ID             int64
	SuperuserID    int64
	OTPHash        string
	ResetTokenHash *string
	ExpiresAt      int64
	VerifiedAt     *int64
	ConsumedAt     *int64
	Attempts       int
	CreatedAt      int64
	UpdatedAt      int64
}
