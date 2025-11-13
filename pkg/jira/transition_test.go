package jira

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransitions(t *testing.T) {
	var (
		apiVersion2          bool
		unexpectedStatusCode bool
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiVersion2 {
			assert.Equal(t, "/rest/api/2/issue/TEST/transitions", r.URL.Path)
		} else {
			assert.Equal(t, "/rest/api/3/issue/TEST/transitions", r.URL.Path)
		}

		assert.Equal(t, "GET", r.Method)

		if unexpectedStatusCode {
			w.WriteHeader(400)
		} else {
			resp, err := os.ReadFile("./testdata/transitions.json")
			assert.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write(resp)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.Transitions("TEST")
	assert.NoError(t, err)

	expected := []*Transition{
		{
			ID:          "11",
			Name:        "To Do",
			IsAvailable: true,
		},
		{
			ID:          "21",
			Name:        "In Progress",
			IsAvailable: true,
		},
		{
			ID:          "31",
			Name:        "Done",
			IsAvailable: false,
		},
	}
	assert.Equal(t, expected, actual)

	apiVersion2 = true
	unexpectedStatusCode = true

	_, err = client.TransitionsV2("TEST")
	assert.Error(t, &ErrUnexpectedResponse{}, err)
}

func TestTransition(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/2/issue/TEST/transitions", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		actualBody := new(strings.Builder)
		_, _ = io.Copy(actualBody, r.Body)

		expectedBody := `{"transition":{"id":"31","name":"Done"}}`
		assert.Equal(t, expectedBody, actualBody.String())

		if unexpectedStatusCode {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	requestData := TransitionRequest{Transition: &TransitionRequestData{
		ID:   "31",
		Name: "Done",
	}}
	code, err := client.Transition("TEST", &requestData)
	assert.NoError(t, err)
	assert.Equal(t, code, 204)
}

func TestTransitionFieldsMarshaler(t *testing.T) {
	fields := TransitionRequestFields{
		Assignee: &struct{ Name string `json:"name"` }{Name: "john"},
		Resolution: &struct{ Name string `json:"name"` }{Name: "Fixed"},
	}

	customFields := customField{
		"customfield_10001": "test-value",
		"customfield_10002": customFieldTypeNumber(5),
		"customfield_10003": customFieldTypeOption{Value: "production"},
	}

	marshaler := NewTransitionFieldsMarshaler(fields, customFields)
	data, err := json.Marshal(marshaler)

	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Check standard fields
	assert.Equal(t, "john", result["assignee"].(map[string]interface{})["name"])
	assert.Equal(t, "Fixed", result["resolution"].(map[string]interface{})["name"])

	// Check custom fields
	assert.Equal(t, "test-value", result["customfield_10001"])
	assert.Equal(t, float64(5), result["customfield_10002"])
	assert.Equal(t, "production", result["customfield_10003"].(map[string]interface{})["value"])
}

func TestTransitionFieldsMarshalerWithoutCustomFields(t *testing.T) {
	fields := TransitionRequestFields{
		Assignee: &struct{ Name string `json:"name"` }{Name: "jane"},
	}

	marshaler := NewTransitionFieldsMarshaler(fields, nil)
	data, err := json.Marshal(marshaler)

	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Check only standard fields are present
	assert.Equal(t, "jane", result["assignee"].(map[string]interface{})["name"])
	assert.Nil(t, result["resolution"])
	assert.Nil(t, result["customfield_10001"])
}

func TestBuildCustomFieldsForTransition(t *testing.T) {
	fields := map[string]string{
		"story-points": "5",
		"environment":  "production",
		"tags":         "bug,urgent",
	}

	configuredFields := []IssueTypeField{
		{
			Key:  "customfield_10001",
			Name: "Story Points",
			Schema: struct {
				DataType string `json:"type"`
				Items    string `json:"items,omitempty"`
			}{DataType: "number"},
		},
		{
			Key:  "customfield_10002",
			Name: "Environment",
			Schema: struct {
				DataType string `json:"type"`
				Items    string `json:"items,omitempty"`
			}{DataType: "option"},
		},
		{
			Key:  "customfield_10003",
			Name: "Tags",
			Schema: struct {
				DataType string `json:"type"`
				Items    string `json:"items,omitempty"`
			}{DataType: "array"},
		},
	}

	result := BuildCustomFieldsForTransition(fields, configuredFields)

	require.NotNil(t, result)
	assert.Equal(t, customFieldTypeNumber(5), result["customfield_10001"])
	assert.Equal(t, customFieldTypeOption{Value: "production"}, result["customfield_10002"])
	assert.Equal(t, []string{"bug", "urgent"}, result["customfield_10003"])
}

func TestBuildCustomFieldsForTransitionWithEmptyFields(t *testing.T) {
	result := BuildCustomFieldsForTransition(map[string]string{}, []IssueTypeField{})
	assert.Nil(t, result)

	result = BuildCustomFieldsForTransition(nil, nil)
	assert.Nil(t, result)
}
