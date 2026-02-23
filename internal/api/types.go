package api

import "encoding/json"

// APIError is returned by Wiro API in errors array.
type APIError struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
	Time    any    `json:"time"`
}

// GenericResponse is common envelope across many endpoints.
type GenericResponse struct {
	Result bool       `json:"result"`
	Errors []APIError `json:"errors"`
}

type AuthSigninResponse struct {
	GenericResponse
	Token               string         `json:"token"`
	VerifyToken         string         `json:"verifytoken"`
	EmailVerifyRequired int            `json:"emailverifyrequired"`
	PhoneVerifyRequired int            `json:"phoneverifyrequired"`
	TwoFactorRequired   int            `json:"twofactorverifyrequired"`
	User                map[string]any `json:"user"`
}

type AuthSigninVerifyResponse struct {
	GenericResponse
	Token string         `json:"token"`
	User  map[string]any `json:"user"`
}

type Project struct {
	ID           string   `json:"id"`
	UUID         string   `json:"uuid"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Slug         *string  `json:"slug"`
	APIKey       string   `json:"apikey"`
	IPWhitelist  []string `json:"ipwhitelist"`
	Time         string   `json:"time"`
	AuthMethod   string   `json:"authmethod"`
	RequestCount string   `json:"requestCount"`
}

type ProjectListResponse struct {
	GenericResponse
	Projects []Project `json:"project"`
}

type ToolOption struct {
	Text  string      `json:"text"`
	Value interface{} `json:"value"`
}

type ToolParameterItem struct {
	Advanced       bool         `json:"advanced"`
	Quick          bool         `json:"quick"`
	Type           string       `json:"type"`
	Class          string       `json:"class"`
	Required       bool         `json:"required"`
	Rows           string       `json:"rows"`
	ID             string       `json:"id"`
	Placeholder    string       `json:"placeholder"`
	Label          string       `json:"label"`
	DefaultValue   interface{}  `json:"defaultvalue"`
	Value          interface{}  `json:"value"`
	MinValue       string       `json:"minvalue"`
	MaxValue       string       `json:"maxvalue"`
	IncrementBy    string       `json:"incrementby"`
	OptionsLoad    string       `json:"optionsLoad"`
	Options        []ToolOption `json:"options"`
	Note           string       `json:"note"`
	MaxInputLenght int          `json:"maxinputlenght"`
}

type ToolParameterGroup struct {
	Title    string              `json:"title"`
	Subtitle string              `json:"subtitle"`
	Items    []ToolParameterItem `json:"items"`
}

type ToolSummary struct {
	ID           string      `json:"id"`
	Title        string      `json:"title"`
	SlugOwner    string      `json:"slugowner"`
	SlugProject  string      `json:"slugproject"`
	Description  string      `json:"description"`
	Image        string      `json:"image"`
	Categories   interface{} `json:"categories"`
	Tags         interface{} `json:"tags"`
	AveragePoint string      `json:"averagepoint"`
	CommentCount string      `json:"commentcount"`
}

type ToolListResponse struct {
	GenericResponse
	Tools []ToolSummary `json:"tool"`
	Total int           `json:"total"`
}

type ToolDetail struct {
	ID           string               `json:"id"`
	Title        string               `json:"title"`
	SlugOwner    string               `json:"slugowner"`
	SlugProject  string               `json:"slugproject"`
	Description  string               `json:"description"`
	Image        string               `json:"image"`
	Categories   []string             `json:"categories"`
	Tags         []string             `json:"tags"`
	Parameters   []ToolParameterGroup `json:"parameters"`
	Inspire      []map[string]any     `json:"inspire"`
	DynamicPrice interface{}          `json:"dynamicprice"`
	Readme       string               `json:"readme"`
}

type ToolDetailResponse struct {
	GenericResponse
	Tools []ToolDetail `json:"tool"`
}

type RunResponse struct {
	GenericResponse
	TaskID            string `json:"taskid"`
	SocketAccessToken string `json:"socketaccesstoken"`
}

type TaskOutput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"contenttype"`
	URL         string `json:"url"`
}

type Task struct {
	ID                string          `json:"id"`
	UUID              string          `json:"uuid"`
	Status            string          `json:"status"`
	SocketAccessToken string          `json:"socketaccesstoken"`
	DebugOutput       string          `json:"debugoutput"`
	DebugError        string          `json:"debugerror"`
	CreateTime        string          `json:"createtime"`
	StartTime         string          `json:"starttime"`
	EndTime           string          `json:"endtime"`
	ParametersRaw     json.RawMessage `json:"parameters"`
	Outputs           []TaskOutput    `json:"outputs"`
}

type TaskDetailResponse struct {
	GenericResponse
	Total    string `json:"total"`
	TaskList []Task `json:"tasklist"`
}
