package entity

import "time"

const (
	StatusAccepted int16 = 1
	StatusEnriched int16 = 2
	StatusInSK     int16 = 3
	StatusFromSK   int16 = 4

	StatusEnrichedName = "обогащена"

	GeoSourceUnknown  int16 = 0
	GeoSourceSupplier int16 = 1
	GeoSourceLookup   int16 = 2

	ChannelSkorozvon int16 = 1
	ChannelPartner   int16 = 2
	ChannelBitrix    int16 = 3
)

type LeadEnrichment struct {
	RegionID       *int
	RegionSource   int16
	OperatorID     *int
	OperatorSource int16
	ChildSourceID  *int
	ColdLabel      string
	BaseLabel      string
	Tag            string
	StatusID       int16
}

type LeadCallResult struct {
	CallID     string
	CallURL    string
	CallLastAt *time.Time
	CallResult *int32
	ProjectNum string
}

type LeadListItem struct {
	ID                int64
	UUID              string
	DateCreate        time.Time
	Source            string
	ChildSource       string
	ColdLabel         string
	BaseLabel         string
	SourceDescription string
	Region            string
	City              string
	Operator          string
	StatusID          int16
	Stage             string
	CallResult        *int32
	UPK               string
	Phone             string
	Zarya             string
	Tag               string
	WebID             string
	ProjectNum        string
	CallID            string
	CallURL           string
}

type LeadEvent struct {
	TypeID     int16
	TypeName   string
	OccurredAt time.Time
	Source     int16
	Details    string
}

type LeadDispatch struct {
	ChannelID   int16
	Channel     string
	UPKID       *int
	UPK         string
	ProjectID   string
	SentAt      time.Time
	SentOK      bool
	ExternalID  string
	Attempts    int16
	HTTPStatus  *int
	RawResponse string
}

type Dispatch struct {
	LeadRowID          int64
	ChannelID          int16
	UPKID              *int
	SkorozvonProjectID string
	ExternalID         string
	SentOK             bool
	Attempts           int16
	HTTPStatus         *int
	RawResponse        string
}

type LeadPhoneHistory struct {
	ID         int64
	UUID       string
	DateCreate time.Time
	Source     string
	StatusID   int16
	Stage      string
}

type LeadDetail struct {
	ID                int64
	UUID              string
	Phone             string
	PhoneValid        bool
	SourceID          *int
	Source            string
	ChildSourceID     *int
	ChildSource       string
	ColdLabel         string
	BaseLabel         string
	RegionID          *int
	Region            string
	RegionSource      *int16
	City              string
	OperatorID        *int
	Operator          string
	OperatorSource    *int16
	WebID             string
	Tag               string
	CallResult        *int32
	CallID            string
	CallURL           string
	CallLastAt        *time.Time
	ProjectNum        string
	CallFormRaw       string
	StatusID          int16
	Stage             string
	S3Key             string
	LegacyRaw         string
	BitrixLeadID      string
	SourceDescription string
	ArchivedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         *time.Time
	Zarya             string
	UPK               string
	Events            []LeadEvent
	Dispatches        []LeadDispatch
	PhoneHistory      []LeadPhoneHistory
}

type LeadSourceCount struct {
	SourceID int `json:"source_id"`
	Count    int `json:"count"`
}

type LeadGeoShare struct {
	Supplier int `json:"supplier"`
	Provider int `json:"provider"`
	Unknown  int `json:"unknown"`
}

type LeadEnrichmentStat struct {
	SourceID int          `json:"source_id"`
	Total    int          `json:"total"`
	Region   LeadGeoShare `json:"region"`
	Operator LeadGeoShare `json:"operator"`
}

type LeadZaryaStat struct {
	SourceID int `json:"source_id"`
	Total    int `json:"total"`
	Unique   int `json:"unique"`
	Dup      int `json:"dup"`
}

type LeadRow struct {
	ID        int64
	UUID      string
	Phone     string
	SourceID  int
	WebID     string
	StatusID  int16
	S3Key     string
	CreatedAt time.Time
}

type OutboxMessage struct {
	ID        int64
	LeadRowID int64
	LeadUUID  string
	SourceID  int
	Payload   string
	CreatedAt time.Time
}
