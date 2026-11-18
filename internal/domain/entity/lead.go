package entity

import (
	"strings"
	"time"
)

const (
	StatusAccepted int16 = 1
	StatusEnriched int16 = 2
	StatusInSK     int16 = 3
	StatusFromSK   int16 = 4
	StatusInB24    int16 = 5
	StatusRejected int16 = 6
	StatusWaiting  int16 = 7

	StatusEnrichedName = "обогащена"

	GeoSourceUnknown  int16 = 0
	GeoSourceSupplier int16 = 1
	GeoSourceLookup   int16 = 2

	ChannelSkorozvon int16 = 1
	ChannelPartner   int16 = 2
	ChannelBitrix    int16 = 3
	ChannelGoogle    int16 = 4
	ChannelCrmcalls  int16 = 5

	EventAccepted int16 = 1
	EventEnriched int16 = 2
	EventInSK     int16 = 3
	EventFromSK   int16 = 4
	EventPartner  int16 = 5
	EventInB24    int16 = 6
	EventRejected int16 = 7
	EventArchived int16 = 8

	EventSourceWorker   int16 = 1
	EventSourceSK       int16 = 2
	EventSourceAPI      int16 = 3
	EventSourceExternal int16 = 4

	// SkorozvonFIOFieldID — custom field «ФИО» в СК. Имя с карточки (lead.name) вяжем сюда.
	SkorozvonFIOFieldID int64 = 50000007333
)

func UsableSkorozvonName(name string) string {
	name = strings.TrimSpace(name)
	switch strings.ToLower(name) {
	case "", "фио не указано", "наименование не указано":
		return ""
	}
	return name
}

type LeadEnrichment struct {
	RegionID       *int
	RegionSource   int16
	OperatorID     *int
	OperatorRaw    string
	OperatorSource int16
	ChildSourceID  *int
	ColdLabel      string
	BaseLabel      string
	Tag            string
	StatusID       int16
}

type LeadCallResult struct {
	CallID         string
	CallURL        string
	CallLastAt     *time.Time
	CallResult     *int64
	CallResultName string
	ProjectNum     string
}

type SkorozvonAnketaAnswer struct {
	QuestionID   int64    `json:"question_id"`
	QuestionName string   `json:"question_name"`
	AnswerID     int64    `json:"answer_id"`
	AnswerValues []string `json:"answer_values"`
}

type LeadFromSkorozvon struct {
	SkorozvonLeadID string
	Phone           string
	City            string
	Region          string
	RegionID        *int
	RegionSource    int16
	Tag             string
	LegacyRaw       string
	CreatedAt       time.Time
	CustomFields    []SkorozvonCustomValue
}

type SkorozvonCustomValue struct {
	FieldID int64
	Raw     []byte
}

type SkorozvonAnketaField struct {
	FieldID   int64
	EnumID    int64
	ValueText string
}

type SkorozvonAnketaFieldItem struct {
	FieldID    int64
	FieldName  string
	BitrixCode string
	EnumID     int64
	EnumName   string
}

type LeadFormResponse struct {
	CallID     string
	UserID     *int64
	FormID     *int64
	ScenarioID *int64
	FormName   string
	UpdatedAt  *time.Time
	Answers    []SkorozvonAnketaAnswer
}

type SkorozvonAnketa struct {
	ID         int64
	UserID     *int64
	FormID     *int64
	ScenarioID *int64
	FormName   string
	CallID     string
	UpdatedAt  *time.Time
	Answers    []SkorozvonAnketaAnswer
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
	CallResult        *int64
	CallResultName    string
	UPK               string
	Phone             string
	Zarya             string
	ZaryaOld          string
	Tag               string
	WebID             string
	ProjectNum        string
	CallID            string
	CallURL           string
	ViaUs             bool
	SkSentOK          *bool
	SkHTTPStatus      *int
	SkError           string
}

type LeadSkDispatch struct {
	LeadRowID  int64
	SentOK     bool
	HTTPStatus *int
	Error      string
}

type LeadSkResend struct {
	ID        int64
	UUID      string
	ProjectID string
}

type LeadEvent struct {
	TypeID     int16
	TypeName   string
	OccurredAt time.Time
	Source     int16
	Details    string
}

type LeadOutboxQueue struct {
	UUID       string
	ChannelID  int16
	Channel    string
	CountRetry int16
	LastRetry  *time.Time
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
	ViaUs             bool
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
	CallResult        *int64
	CallResultName    string
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
	ZaryaOld          string
	UPK               string
	Events            []LeadEvent
	Dispatches        []LeadDispatch
	Queues            []LeadOutboxQueue
	PhoneHistory      []LeadPhoneHistory
	Anketa            *SkorozvonAnketa
	AnketaFields      []SkorozvonAnketaFieldItem
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

type LeadSkorozvonBitrixStat struct {
	SourceID    int `json:"source_id"`
	Updated     int `json:"updated"`
	UpdatedSent int `json:"updated_sent"`
	Created     int `json:"created"`
	CreatedSent int `json:"created_sent"`
}

type LeadPartnerDispatchCount struct {
	PartnerID int `json:"partner_id"`
	Count     int `json:"count"`
}

type LeadSkorozvonDispatchCount struct {
	ProjectID string `json:"project_id"`
	Count     int    `json:"count"`
}

type LeadOutgoingStats struct {
	Partners  []LeadPartnerDispatchCount   `json:"partners"`
	Skorozvon []LeadSkorozvonDispatchCount `json:"skorozvon"`
	Crmcalls  []LeadSkorozvonDispatchCount `json:"crmcalls"`
}

type LeadForSkorozvon struct {
	ID                int64
	UUID              string
	Phone             string
	ChildSourceID     *int
	Region            string
	RegionID          *int
	Zarya             string
	ColdLabel         string
	SourceDescription string
}

type LeadRow struct {
	ID                int64
	UUID              string
	Phone             string
	SourceID          int
	WebID             string
	SourceDescription string
	StatusID          int16
	S3Key             string
	CreatedAt         time.Time
}

type LeadImportRow struct {
	UUID       string
	Phone      string
	PhoneValid bool
	SourceID   int
	Payload    string
	Values     map[string]any
}

type OutboxMessage struct {
	ID        int64
	LeadRowID int64
	LeadUUID  string
	SourceID  int
	Payload   string
	CreatedAt time.Time
}

type OutboxBx24 struct {
	UUID        string
	LeadUUID    string
	LockedUntil *time.Time
	CountRetry  int16
	LastRetry   *time.Time
}

type OutboxSkorozvon struct {
	UUID        string
	LeadUUID    string
	ProjectID   *int64
	LockedUntil *time.Time
	CountRetry  int16
	LastRetry   *time.Time
}

type OutboxCrmcalls struct {
	UUID        string
	LeadUUID    string
	LockedUntil *time.Time
	CountRetry  int16
	LastRetry   *time.Time
}

type CrmcallsPostback struct {
	ID         string
	Name       string
	Phone      string
	Result     string
	CallID     string
	CallURL    string
	CallLastAt *time.Time
}
