package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// TransitionRequest struct holds request data for issue transition request.
type TransitionRequest struct {
	Update     *TransitionRequestUpdate    `json:"update,omitempty"`
	Fields     *transitionFieldsMarshaler  `json:"fields,omitempty"`
	Transition *TransitionRequestData      `json:"transition"`
}

// TransitionRequestUpdate struct holds a list of operations to perform on the issue screen field.
type TransitionRequestUpdate struct {
	Comment []struct {
		Add struct {
			Body string `json:"body"`
		} `json:"add"`
	} `json:"comment,omitempty"`
}

// TransitionRequestFields struct holds a list of issue screen fields to update along with sub-fields.
type TransitionRequestFields struct {
	Assignee *struct {
		Name string `json:"name"`
	} `json:"assignee,omitempty"`
	Resolution *struct {
		Name string `json:"name"`
	} `json:"resolution,omitempty"`

	customFields customField
}

// TransitionRequestData is a transition request data.
type TransitionRequestData struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// transitionFieldsMarshaler is a custom marshaler to handle custom fields.
type transitionFieldsMarshaler struct {
	M TransitionRequestFields
}

// MarshalJSON is a custom marshaler to handle dynamic custom fields.
func (tfm transitionFieldsMarshaler) MarshalJSON() ([]byte, error) {
	// Marshal the struct normally
	m, err := json.Marshal(tfm.M)
	if err != nil {
		return m, err
	}

	// Unmarshal to map so we can inject custom fields
	var temp interface{}
	if err := json.Unmarshal(m, &temp); err != nil {
		return nil, err
	}
	dm := temp.(map[string]interface{})

	// Inject custom fields into the map
	for key, val := range tfm.M.customFields {
		dm[key] = val
	}

	return json.Marshal(dm)
}

type transitionResponse struct {
	Expand      string        `json:"expand"`
	Transitions []*Transition `json:"transitions"`
}

// Transitions fetches valid transitions for an issue using v3 version of the GET /issue/{key}/transitions endpoint.
func (c *Client) Transitions(key string) ([]*Transition, error) {
	return c.transitions(key, apiVersion3)
}

// TransitionsV2 fetches valid transitions for an issue using v2 version of the GET /issue/{key}/transitions endpoint.
func (c *Client) TransitionsV2(key string) ([]*Transition, error) {
	return c.transitions(key, apiVersion2)
}

func (c *Client) transitions(key, ver string) ([]*Transition, error) {
	path := fmt.Sprintf("/issue/%s/transitions", key)

	var (
		res *http.Response
		err error
	)

	switch ver {
	case apiVersion2:
		res, err = c.GetV2(context.Background(), path, nil)
	default:
		res, err = c.Get(context.Background(), path, nil)
	}

	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, formatUnexpectedResponse(res)
	}

	var out transitionResponse

	err = json.NewDecoder(res.Body).Decode(&out)

	return out.Transitions, err
}

// Transition moves issue from one state to another using POST /issue/{key}/transitions endpoint.
func (c *Client) Transition(key string, data *TransitionRequest) (int, error) {
	body, err := json.Marshal(&data)
	if err != nil {
		return 0, err
	}

	path := fmt.Sprintf("/issue/%s/transitions", key)

	res, err := c.PostV2(context.Background(), path, body, Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	if err != nil {
		return 0, err
	}
	if res == nil {
		return 0, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		return res.StatusCode, formatUnexpectedResponse(res)
	}
	return res.StatusCode, nil
}

// NewTransitionFieldsMarshaler creates a new transition fields marshaler with custom fields.
func NewTransitionFieldsMarshaler(fields TransitionRequestFields, customFields customField) *transitionFieldsMarshaler {
	fields.customFields = customFields
	return &transitionFieldsMarshaler{M: fields}
}

// BuildCustomFieldsForTransition constructs custom fields map for transitions.
// This is extracted from constructCustomFields() in create.go.
func BuildCustomFieldsForTransition(fields map[string]string, configuredFields []IssueTypeField) customField {
	if len(fields) == 0 || len(configuredFields) == 0 {
		return nil
	}

	cf := make(customField)

	for key, val := range fields {
		for _, configured := range configuredFields {
			identifier := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(configured.Name)), " ", "-")
			if identifier != strings.ToLower(key) {
				continue
			}

			switch configured.Schema.DataType {
			case customFieldFormatOption:
				cf[configured.Key] = customFieldTypeOption{Value: val}
			case customFieldFormatProject:
				cf[configured.Key] = customFieldTypeProject{Value: val}
			case customFieldFormatArray:
				pieces := strings.Split(strings.TrimSpace(val), ",")
				if configured.Schema.Items == customFieldFormatOption {
					items := make([]customFieldTypeOption, 0)
					for _, p := range pieces {
						items = append(items, customFieldTypeOption{Value: strings.TrimSpace(p)})
					}
					cf[configured.Key] = items
				} else {
					trimmed := make([]string, 0, len(pieces))
					for _, p := range pieces {
						trimmed = append(trimmed, strings.TrimSpace(p))
					}
					cf[configured.Key] = trimmed
				}
			case customFieldFormatNumber:
				num, err := strconv.ParseFloat(val, 64)
				if err != nil {
					cf[configured.Key] = val
				} else {
					cf[configured.Key] = customFieldTypeNumber(num)
				}
			default:
				cf[configured.Key] = val
			}
		}
	}

	return cf
}
