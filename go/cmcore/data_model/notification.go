package datamodel

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

var allDaysOfWeek = []time.Weekday{
	time.Sunday,
	time.Monday,
	time.Tuesday,
	time.Wednesday,
	time.Thursday,
	time.Friday,
	time.Saturday,
}

const defaultDeliveryWindowLocalTimeStart = 0
const defaultDeliveryWindowLocalTimeEnd = 23*60 + 59

var validInterruptionLevels = []string{"active", "critical", "passive", "timeSensitive"}

type Notification struct {
	ID         string
	Title      string
	Body       string
	BadgeCount int
	ActionName string
	Sound      string

	LaunchImageName string

	RelevanceScore    *float64
	InterruptionLevel string

	DeliveryTime                  DeliveryTime
	DeliveryDaysOfWeek            []time.Weekday
	DeliveryWindowTODStartMinutes int
	DeliveryWindowTODEndMinutes   int

	IdealDevlieryConditions *IdealDevlieryConditions
	CancelationEvents       *[]string
}

type IdealDevlieryConditions struct {
	Condition   Condition `json:"condition"`
	MaxWaitTime int       `json:"maxWaitTime"`
}

type EventInstanceTypeEnum int

const (
	// unknown, should not process. Could be type from future SDK
	EventInstanceTypeUnknown EventInstanceTypeEnum = iota
	// The notification's time is relative to the latest time the event occured
	EventInstanceTypeLatest
	// The notification's time is relative to the first time the event occured
	EventInstanceTypeFirst
)

type DeliveryTime struct {
	TimestampEpoch      *int64  `json:"timestamp,omitempty"`
	EventName           *string `json:"eventName,omitempty"`
	EventOffset         *int    `json:"eventOffset,omitempty"`
	EventInstanceString *string `json:"eventInstance,omitempty"`
}

func (dt *DeliveryTime) EventInstance() EventInstanceTypeEnum {
	// Default to latest for nil/empty, but not unrecognized
	if dt.EventInstanceString == nil || *dt.EventInstanceString == "" || *dt.EventInstanceString == "latest" {
		return EventInstanceTypeLatest
	}
	if dt.EventInstanceString != nil && *dt.EventInstanceString == "first" {
		return EventInstanceTypeFirst
	}
	return EventInstanceTypeUnknown
}

type jsonNotification struct {
	Title      string `json:"title,omitempty"`
	Body       string `json:"body,omitempty"`
	BadgeCount *int   `json:"badgeCount,omitempty"`
	ActionName string `json:"tapActionName,omitempty"`
	Sound      string `json:"sound,omitempty"`

	LaunchImageName string `json:"launchImageName,omitempty"`

	RelevanceScore    *float64 `json:"relevanceScore,omitempty"`
	InterruptionLevel string   `json:"interruptionLevel,omitempty"`

	DeliveryTime             DeliveryTime `json:"deliveryTime,omitempty"`
	DeliveryDaysOfWeekString string       `json:"deliveryDaysOfWeek,omitempty"`
	// HH:MM format
	DeliveryWindowTODStart string `json:"deliveryTimeOfDayStart,omitempty"`
	DeliveryWindowTODEnd   string `json:"deliveryTimeOfDayEnd,omitempty"`

	IdealDeliveryConditions *IdealDevlieryConditions `json:"idealDeliveryConditions,omitempty"`
	CancelationEvents       *[]string                `json:"cancelationEvents,omitempty"`
}

func (a *Notification) Validate() bool {
	return a.ValidateReturningUserReadableIssue() == ""
}

func (n *Notification) ValidateReturningUserReadableIssue() string {
	return n.ValidateReturningUserReadableIssueIgnoreID(false)
}

func (n *Notification) ValidateReturningUserReadableIssueIgnoreID(ignoreID bool) string {
	if !ignoreID && n.ID == "" {
		return "Notification must have ID"
	}
	if n.Title == "" &&
		n.Body == "" &&
		n.BadgeCount < 0 {
		return "Notifications must have one or more of: title, body, and badgeCount."
	}
	if n.DeliveryWindowTODEndMinutes < 0 || n.DeliveryWindowTODEndMinutes > 24*60-1 {
		return "Notifications must have a deliveryTimeOfDayStart between 0 and 24*60 mins."
	}
	if n.DeliveryWindowTODEndMinutes < 0 || n.DeliveryWindowTODEndMinutes > 24*60-1 {
		return "Notifications must have a deliveryTimeOfDayEnd between 0 and 24*60 mins."
	}
	if n.DeliveryWindowTODStartMinutes > n.DeliveryWindowTODEndMinutes {
		return "Notifications must have a deliveryTimeOfDayStart before deliveryTimeOfDayEnd."
	}
	if len(n.DeliveryDaysOfWeek) == 0 {
		return "Notifications must have at least one day of week valid for delivery."
	}
	if n.RelevanceScore != nil && (*n.RelevanceScore < 0 || *n.RelevanceScore > 1) {
		return "Relevance score must be between 0 and 1 if provided."
	}
	if StrictDatamodelParsing && n.InterruptionLevel != "" {
		if !slices.Contains(validInterruptionLevels, n.InterruptionLevel) {
			return fmt.Sprintf("Interruption level must be one of %v, got %v", validInterruptionLevels, n.InterruptionLevel)
		}
	}
	if n.CancelationEvents != nil {
		for _, event := range *n.CancelationEvents {
			if event == "" {
				return fmt.Sprintf("Notification '%v' has an blank cancelation event", n.ID)
			}
		}
	}
	if n.IdealDevlieryConditions != nil {
		if conErr := n.IdealDevlieryConditions.Condition.Validate(); conErr != nil {
			conErrUserReadable := "Unknown error"
			if uperr, ok := conErr.(*UserPresentableError); ok {
				conErrUserReadable = uperr.UserErrorString()
			}
			return "Notification has invalid ideal delivery condition: " + conErrUserReadable
		}
		if n.IdealDevlieryConditions.MaxWaitTime < -1 || n.IdealDevlieryConditions.MaxWaitTime == 0 {
			return "Notifications must have a max wait time for ideal delivery condition. Valid values are -1 (forever) or values greater than 0."
		}
	}
	if dtErr := n.DeliveryTime.ValidateReturningUserReadableIssue(); dtErr != "" {
		return "Notification has invalid delivery time: " + dtErr
	}
	return ""
}

func (d *DeliveryTime) ValidateReturningUserReadableIssue() string {
	if d.TimestampEpoch == nil && d.EventName == nil {
		return "DeliveryTime must have either a Timestamp or an EventName defined."
	}
	if d.TimestampEpoch != nil && d.EventName != nil {
		return "DeliveryTime cannot have both a Timestamp and an EventName defined."
	}
	if d.TimestampEpoch != nil && d.EventOffset != nil {
		return "DeliveryTime cannot have both a Timestamp and an EventOffset defined."
	}
	if StrictDatamodelParsing {
		if d.EventInstance() == EventInstanceTypeUnknown {
			return fmt.Sprintf("Notification event instance must be 'first' or 'latest', got '%v'", d.EventInstanceString)
		}
	}
	return ""
}

func (d *DeliveryTime) Timestamp() *time.Time {
	if d.TimestampEpoch == nil {
		return nil
	}
	time := time.Unix(*d.TimestampEpoch, 0)
	return &time
}

func (n *Notification) UnmarshalJSON(data []byte) error {
	var jn jsonNotification
	err := json.Unmarshal(data, &jn)
	if err != nil {
		return NewUserPresentableErrorWSource("Unable to parse the json of an notification with type=notification. Check the format, variable names, and types (eg float vs int).", err)
	}

	n.Title = jn.Title
	n.Body = jn.Body
	n.Sound = jn.Sound
	n.ActionName = jn.ActionName
	n.IdealDevlieryConditions = jn.IdealDeliveryConditions
	n.CancelationEvents = jn.CancelationEvents
	n.DeliveryTime = jn.DeliveryTime
	n.RelevanceScore = jn.RelevanceScore
	n.InterruptionLevel = jn.InterruptionLevel
	n.LaunchImageName = jn.LaunchImageName

	if jn.BadgeCount != nil {
		n.BadgeCount = *jn.BadgeCount
		if StrictDatamodelParsing && n.BadgeCount < 0 {
			return NewUserPresentableError("Notification badgeCount must be greater than or equal to 0")
		}
	} else {
		n.BadgeCount = -1 // default to -1 for unset
	}
	if jn.DeliveryDaysOfWeekString != "" {
		n.DeliveryDaysOfWeek = parseDaysOfWeekString(jn.DeliveryDaysOfWeekString)
	} else {
		// Default to all days of week
		n.DeliveryDaysOfWeek = allDaysOfWeek
	}

	// Defaults could change over time, so either all custom, or all default or config could be invalid
	if (jn.DeliveryWindowTODStart == "" && jn.DeliveryWindowTODEnd != "") ||
		(jn.DeliveryWindowTODStart != "" && jn.DeliveryWindowTODEnd == "") {
		return NewUserPresentableError("DeliveryTime must have both deliveryTimeOfDayStart and deliveryTimeOfDayEnd defined if either is defined.")
	}
	if jn.DeliveryWindowTODStart != "" {
		deliveryStart, err := parseMinutesFromHHMMString(jn.DeliveryWindowTODStart)
		if err != nil && StrictDatamodelParsing {
			return NewUserPresentableError("Invalid deliveryTimeOfDayStart. Expect HH:MM format. Was: " + jn.DeliveryWindowTODStart)
		} else if err != nil {
			fmt.Printf("CriticalMoments: invalid deliveryTimeOfDayStart [%v]. Using default: %v\n", jn.DeliveryWindowTODStart, defaultDeliveryWindowLocalTimeStart)
			n.DeliveryWindowTODStartMinutes = defaultDeliveryWindowLocalTimeStart
		} else {
			n.DeliveryWindowTODStartMinutes = deliveryStart
		}
	} else {
		n.DeliveryWindowTODStartMinutes = defaultDeliveryWindowLocalTimeStart
	}
	if jn.DeliveryWindowTODEnd != "" {
		deliveryEnd, err := parseMinutesFromHHMMString(jn.DeliveryWindowTODEnd)
		if err != nil && StrictDatamodelParsing {
			return NewUserPresentableError("Invalid deliveryTimeOfDayEnd. Expect HH:MM format. Was: " + jn.DeliveryWindowTODEnd)
		} else if err != nil {
			fmt.Printf("CriticalMoments: invalid deliveryTimeOfDayEnd [%v]. Using default: %v\n", jn.DeliveryWindowTODEnd, defaultDeliveryWindowLocalTimeEnd)
			n.DeliveryWindowTODEndMinutes = defaultDeliveryWindowLocalTimeEnd
		} else {
			n.DeliveryWindowTODEndMinutes = deliveryEnd
		}
	} else {
		n.DeliveryWindowTODEndMinutes = defaultDeliveryWindowLocalTimeEnd
	}

	// ignore ID which is set later from primary config
	if validationIssue := n.ValidateReturningUserReadableIssueIgnoreID(true); validationIssue != "" {
		return NewUserPresentableError(validationIssue)
	}

	return nil
}

func parseMinutesFromHHMMString(i string) (int, error) {
	if i == "" {
		return 0, errors.New("Empty string for HH:MM time of day")
	}
	values := strings.Split(i, ":")
	if len(values) != 2 {
		return 0, errors.New("invalid HH:MM time of day")
	}
	hours, err := strconv.Atoi(values[0])
	if err != nil {
		return 0, err
	}
	minutes, err := strconv.Atoi(values[1])
	if err != nil {
		return 0, err
	}
	if hours < 0 || hours > 23 || minutes < 0 || minutes > 59 {
		return 0, errors.New("invalid HH:MM time of day")
	}
	return hours*60 + minutes, nil
}

// Parsed comman separated day of week strings, removing dupes and standardizing order
func parseDaysOfWeekString(i string) []time.Weekday {
	days := []time.Weekday{}
	components := strings.Split(i, ",")

	for dow := range allDaysOfWeek {
		dowWD := time.Weekday(dow)
		dowName := dowWD.String()
		dayFound := false
		for _, dayStringComponent := range components {
			if dowName == dayStringComponent {
				dayFound = true
			}
		}
		if dayFound {
			days = append(days, dowWD)
		}
	}

	return days
}

const NotificationUniqueIDPrefix = "io.criticalmoments.notifications."

func (n *Notification) UniqueID() string {
	return fmt.Sprintf("%v%v", NotificationUniqueIDPrefix, n.ID)
}

// Gomobile accessor (doesn't support pointers)
func (n *Notification) GetRelevanceScore() float64 {
	return *n.RelevanceScore
}

func (n *Notification) HasRelevanceScore() bool {
	return n.RelevanceScore != nil
}
