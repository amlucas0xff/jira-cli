package cmdcommon

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ankitpokhrel/jira-cli/api"
	"github.com/ankitpokhrel/jira-cli/internal/cmdutil"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
)

const (
	// ActionSubmit is a submit action.
	ActionSubmit = "Submit"
	// ActionCancel is a cancel action.
	ActionCancel = "Cancel"
	// ActionMetadata is an add metadata action.
	ActionMetadata = "Add metadata"
)

// CreateParams holds parameters for create command.
type CreateParams struct {
	Name             string
	IssueType        string
	IssueTypeID      string // ID of the issue type, used for API calls
	ParentIssueKey   string
	Summary          string
	Body             string
	Priority         string
	Reporter         string
	Assignee         string
	Labels           []string
	Components       []string
	FixVersions      []string
	AffectsVersions  []string
	OriginalEstimate string
	CustomFields     map[string]string
	Template         string
	NoInput          bool
	Debug            bool
}

// SetCreateFlags sets flags supported by create command.
func SetCreateFlags(cmd *cobra.Command, prefix string) {
	custom := make(map[string]string)

	cmd.Flags().SortFlags = false

	if prefix == "Epic" {
		cmd.Flags().StringP("name", "n", "", "Epic name")
	} else {
		cmd.Flags().StringP("type", "t", "", "Issue type")
		cmd.Flags().StringP("parent", "P", "", `Parent issue key can be used to attach epic to an issue.
And, this field is mandatory when creating a sub-task.`)
	}
	cmd.Flags().StringP("summary", "s", "", prefix+" summary or title")
	cmd.Flags().StringP("body", "b", "", prefix+" description")
	cmd.Flags().StringP("priority", "y", "", prefix+" priority")
	cmd.Flags().StringP("reporter", "r", "", prefix+" reporter (username, email or display name)")
	cmd.Flags().StringP("assignee", "a", "", prefix+" assignee (username, email or display name)")
	cmd.Flags().StringArrayP("label", "l", []string{}, prefix+" labels")
	cmd.Flags().StringArrayP("component", "C", []string{}, prefix+" components")
	cmd.Flags().StringArray("fix-version", []string{}, "Release info (fixVersions)")
	cmd.Flags().StringArray("affects-version", []string{}, "Release info (affectsVersions)")
	cmd.Flags().StringP("original-estimate", "e", "", prefix+" Original estimate")
	cmd.Flags().StringToString("custom", custom, "Set custom fields")
	cmd.Flags().StringP("template", "T", "", "Path to a file to read body/description from")
	cmd.Flags().Bool("web", false, "Open in web browser after successful creation")
	cmd.Flags().Bool("no-input", false, "Disable prompt for non-required fields")
}

// GetNextAction provide user an option to select next action.
func GetNextAction() *survey.Question {
	return &survey.Question{
		Name: "action",
		Prompt: &survey.Select{
			Message: "What's next?",
			Options: []string{
				ActionSubmit,
				ActionMetadata,
				ActionCancel,
			},
		},
		Validate: survey.Required,
	}
}

// GetMetadata gathers a list of metadata users wants to add.
func GetMetadata() []*survey.Question {
	return []*survey.Question{
		{
			Name: "metadata",
			Prompt: &survey.MultiSelect{
				Message: "What would you like to add?",
				Options: []string{"Priority", "Components", "Labels", "FixVersions", "AffectsVersions"},
			},
		},
	}
}

// GetMetadataQuestions prepares metadata question to input from user.
func GetMetadataQuestions(cat []string) []*survey.Question {
	var qs []*survey.Question

	for _, c := range cat {
		switch c {
		case "Priority":
			qs = append(qs, &survey.Question{
				Name:   "priority",
				Prompt: &survey.Input{Message: "Priority"},
			})
		case "Components":
			qs = append(qs, &survey.Question{
				Name: "components",
				Prompt: &survey.Input{
					Message: "Components",
					Help:    "Comma separated list of valid components. For eg: BE,FE",
				},
			})
		case "Labels":
			qs = append(qs, &survey.Question{
				Name: "labels",
				Prompt: &survey.Input{
					Message: "Labels",
					Help:    "Comma separated list of labels. For eg: backend,urgent",
				},
			})
		case "FixVersions":
			qs = append(qs, &survey.Question{
				Name: "fixversions",
				Prompt: &survey.Input{
					Message: "Fix Versions",
					Help:    "Comma separated list of fixVersions. For eg: v1.0-beta,v2.0",
				},
			})
		case "AffectsVersions":
			qs = append(qs, &survey.Question{
				Name: "affectsversions",
				Prompt: &survey.Input{
					Message: "Affects Versions",
					Help:    "Comma separated list of affectsVersions. For eg: v1.0-beta,v2.0",
				},
			})
		}
	}

	return qs
}

// HandleNoInput handles operations for --no-input flag.
func HandleNoInput(params *CreateParams) error {
	answer := struct{ Action string }{}
	for answer.Action != ActionSubmit {
		err := survey.Ask([]*survey.Question{GetNextAction()}, &answer)
		if err != nil {
			return err
		}

		switch answer.Action {
		case ActionCancel:
			cmdutil.Failed("Action aborted")
		case ActionMetadata:
			ans := struct{ Metadata []string }{}
			err := survey.Ask(GetMetadata(), &ans)
			if err != nil {
				return err
			}

			if len(ans.Metadata) > 0 {
				qs := GetMetadataQuestions(ans.Metadata)
				ans := struct {
					Priority        string
					Labels          string
					Components      string
					FixVersions     string
					AffectsVersions string
				}{}
				err := survey.Ask(qs, &ans)
				if err != nil {
					return err
				}

				if ans.Priority != "" {
					params.Priority = ans.Priority
				}
				if len(ans.Labels) > 0 {
					params.Labels = strings.Split(ans.Labels, ",")
				}
				if len(ans.Components) > 0 {
					params.Components = strings.Split(ans.Components, ",")
				}
				if len(ans.FixVersions) > 0 {
					params.FixVersions = strings.Split(ans.FixVersions, ",")
				}
				if len(ans.AffectsVersions) > 0 {
					params.AffectsVersions = strings.Split(ans.AffectsVersions, ",")
				}
			}
		}
	}
	return nil
}

// GetRelevantUser finds and returns a valid user name or account ID based on user input.
func GetRelevantUser(client *jira.Client, project string, user string) string {
	if user == "" {
		return ""
	}
	u, err := api.ProxyUserSearch(client, &jira.UserSearchOptions{
		Query:   user,
		Project: project,
	})
	if err != nil || len(u) == 0 {
		cmdutil.Failed("Unable to find associated user for %s", user)
	}
	return GetUserKeyForConfiguredInstallation(u[0])
}

// GetUserKeyForConfiguredInstallation returns either the user name or account ID based on jira installation type.
func GetUserKeyForConfiguredInstallation(user *jira.User) string {
	it := viper.GetString("installation")
	if it == jira.InstallationTypeLocal {
		return user.Name
	}
	return user.AccountID
}

// GetConfiguredCustomFields returns the custom fields configured by the user.
func GetConfiguredCustomFields() ([]jira.IssueTypeField, error) {
	var configuredFields []jira.IssueTypeField

	err := viper.UnmarshalKey("issue.fields.custom", &configuredFields)
	if err != nil {
		return nil, err
	}

	return configuredFields, nil
}

// ValidateCustomFields validates custom fields.
// TODO: Fail with error instead of warning in future release.
func ValidateCustomFields(fields map[string]string, configuredFields []jira.IssueTypeField) {
	if len(fields) == 0 {
		return
	}

	fieldsMap := make(map[string]string)
	for _, configured := range configuredFields {
		identifier := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(configured.Name)), " ", "-")
		fieldsMap[identifier] = configured.Name
	}

	invalidCustomFields := make([]string, 0, len(fields))
	for key := range fields {
		if _, ok := fieldsMap[key]; !ok {
			invalidCustomFields = append(invalidCustomFields, key)
		}
	}

	if len(invalidCustomFields) > 0 {
		cmdutil.Warn(`
Some custom fields are not configured and will be ignored. This will fail with error in the future release.
Please make sure that the passed custom fields are valid and configured accordingly in the config file.
Invalid custom fields used in the command: %s`,
			strings.Join(invalidCustomFields, ", "),
		)
	}
}

// ValidateAndFilterCustomFields validates custom fields against available fields for an issue type.
// Returns filtered valid fields and an error if any invalid fields are found.
func ValidateAndFilterCustomFields(
	requested map[string]string,
	available []jira.IssueTypeField,
	configuredFields []jira.IssueTypeField,
	issueTypeName string,
) (map[string]string, error) {
	if len(requested) == 0 {
		return requested, nil
	}

	// Build map of configured field names to field keys for name normalization
	configuredMap := make(map[string]string)
	for _, field := range configuredFields {
		identifier := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(field.Name)), " ", "-")
		configuredMap[identifier] = field.Key
	}

	// Build map of available field keys for this issue type
	availableMap := make(map[string]jira.IssueTypeField)
	for _, field := range available {
		availableMap[field.Key] = field
	}

	validFields := make(map[string]string)
	invalidFields := make([]string, 0)
	var invalidFieldKeys []string

	for requestedName, value := range requested {
		// Get the field key from configured fields
		fieldKey, exists := configuredMap[strings.ToLower(requestedName)]
		if !exists {
			invalidFields = append(invalidFields, requestedName)
			continue
		}

		// Check if this field key is available for the issue type
		if _, available := availableMap[fieldKey]; available {
			validFields[requestedName] = value
		} else {
			invalidFields = append(invalidFields, requestedName)
			invalidFieldKeys = append(invalidFieldKeys, fieldKey)
		}
	}

	if len(invalidFields) > 0 {
		// Build helpful error message with available custom fields
		availableCustomFields := make([]string, 0)
		for _, field := range available {
			// Only show custom fields (they start with "customfield_")
			if strings.HasPrefix(field.Key, "customfield_") && field.Name != "" {
				identifier := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(field.Name)), " ", "-")
				availableCustomFields = append(availableCustomFields, identifier)
			}
		}

		errMsg := fmt.Sprintf(
			"Invalid custom fields for issue type '%s': %s\n\n"+
				"These fields are not available on the create/edit screen for this issue type.\n"+
				"This is a Jira project configuration issue, not a CLI problem.\n\n",
			issueTypeName,
			strings.Join(invalidFields, ", "),
		)

		if len(invalidFieldKeys) > 0 {
			errMsg += fmt.Sprintf("Field IDs: %s\n\n", strings.Join(invalidFieldKeys, ", "))
		}

		if len(availableCustomFields) > 0 {
			errMsg += fmt.Sprintf(
				"Available custom fields for '%s':\n  %s\n\n",
				issueTypeName,
				strings.Join(availableCustomFields, "\n  "),
			)
		} else {
			errMsg += "No custom fields are available for this issue type.\n\n"
		}

		errMsg += "To fix this:\n" +
			"1. Check your Jira project's screen configuration\n" +
			"2. Add the required fields to the issue type's create screen\n" +
			"3. Or use only the fields listed as available above"

		return nil, fmt.Errorf(errMsg)
	}

	return validFields, nil
}
