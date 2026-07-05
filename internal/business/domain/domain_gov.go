// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// Иргэний "Төрийн үйлчилгээ" порталын домэйн entity-үүд (me систем). gov_services
// нь нийтийн каталог; бусад нь хэрэглэгч-тус-бүрийн (UserID-гаар scope хийгдэнэ).

// GovService нь каталогийн нэг үйлчилгээ.
type GovService struct {
	ID             string
	Code           string
	Name           string
	Category       string
	Agency         string
	Description    string
	Fee            int // MNT
	ProcessingDays int
	Online         bool
	Enabled        bool
	CreatedAt      time.Time
}

// GovApplication нь иргэний үйлчилгээний хүсэлт.
type GovApplication struct {
	ID          string
	UserID      string
	ServiceID   *string
	ServiceName string
	ReferenceNo string
	Status      string
	Note        string
	SubmittedAt time.Time
	UpdatedAt   *time.Time
}

// GovReference нь олгогдсон лавлагаа/тодорхойлолт.
type GovReference struct {
	ID          string
	UserID      string
	Type        string
	Title       string
	ReferenceNo string
	Status      string
	IssuedAt    time.Time
	ValidUntil  *time.Time
	Data        []byte // jsonb
}

// GovNotification нь иргэнд илгээсэн мэдэгдэл.
type GovNotification struct {
	ID        string
	UserID    string
	Title     string
	Body      string
	Category  string
	Read      bool
	CreatedAt time.Time
}

// GovPayment нь төлбөр (татвар/хураамж/торгууль).
type GovPayment struct {
	ID        string
	UserID    string
	Title     string
	Category  string
	Amount    int
	Currency  string
	Status    string
	DueDate   *time.Time
	PaidAt    *time.Time
	CreatedAt time.Time
}

// GovAppointment нь төрийн байгууллага дахь цаг захиалга.
type GovAppointment struct {
	ID          string
	UserID      string
	ServiceID   *string
	ServiceName string
	Agency      string
	Location    string
	ScheduledAt time.Time
	Status      string
	Note        string
	CreatedAt   time.Time
}

// GovOverview нь иргэний нүүр хуудасны нэгтгэл.
type GovOverview struct {
	OpenApplications     int
	UnreadNotifications  int
	UnpaidCount          int
	UnpaidAmount         int
	UpcomingCount        int
	IssuedReferences     int
	RecentApplications   []GovApplication
	UpcomingAppointments []GovAppointment
}
