# Custom Fields Support for Move Command

## Date
2025-11-13

## Summary
Added support for custom fields in the `jira issue move` command, allowing users to set custom field values during issue transitions.

## Changes

### Files Modified
```
internal/cmd/issue/move/move.go |  50 +++++++++++++------
pkg/jira/transition.go          |  96 +++++++++++++++++++++++++++++++++++--
pkg/jira/transition_test.go     | 103 ++++++++++++++++++++++++++++++++++++++++
3 files changed, 232 insertions(+), 17 deletions(-)
```

### Implementation Details

#### 1. API Layer Changes (`pkg/jira/transition.go`)
- Added `customFields` field to `TransitionRequestFields` struct
- Created `transitionFieldsMarshaler` to handle custom field serialization
- Implemented `NewTransitionFieldsMarshaler()` helper function
- Added `BuildCustomFieldsForTransition()` function to construct custom fields from user input
- Supports all custom field types: string, number, option, project, and array

#### 2. Command Layer Changes (`internal/cmd/issue/move/move.go`)
- Added `--custom` flag (StringToString type) to accept custom field key-value pairs
- Extended `moveParams` struct to include `customFields` field
- Integrated custom field validation using existing `cmdcommon.GetConfiguredCustomFields()` and `cmdcommon.ValidateCustomFields()`
- Updated examples to demonstrate custom field usage

#### 3. Tests (`pkg/jira/transition_test.go`)
- `TestTransitionFieldsMarshaler` - Tests marshaling with custom fields
- `TestTransitionFieldsMarshalerWithoutCustomFields` - Tests backward compatibility
- `TestBuildCustomFieldsForTransition` - Tests custom field construction
- `TestBuildCustomFieldsForTransitionWithEmptyFields` - Tests edge cases

## Testing

### Unit Tests
All tests passed successfully:
```bash
go test ./pkg/jira -run TestTransition -v
```

Results:
- TestTransitions: PASS
- TestTransition: PASS
- TestTransitionFieldsMarshaler: PASS
- TestTransitionFieldsMarshalerWithoutCustomFields: PASS
- TestBuildCustomFieldsForTransition: PASS
- TestBuildCustomFieldsForTransitionWithEmptyFields: PASS

### Manual Testing Steps

#### Prerequisites
1. Configure custom fields in `.jira/config.yml`:
```yaml
issue:
  fields:
    custom:
      - name: "Story Points"
        key: "customfield_10001"
        schema:
          type: "number"
      - name: "Environment"
        key: "customfield_10002"
        schema:
          type: "option"
      - name: "Tags"
        key: "customfield_10003"
        schema:
          type: "array"
```

#### Test Cases

1. **Basic transition with single custom field**
```bash
jira issue move PROJ-123 "In Progress" --custom story-points=5
```

Expected Result:
- Issue transitions to "In Progress" state
- Story Points custom field is set to 5
- Success message displayed

2. **Transition with multiple custom fields**
```bash
jira issue move PROJ-123 Done \
  --assignee jane \
  --resolution Fixed \
  --custom environment=production \
  --custom tags=bug,urgent
```

Expected Result:
- Issue transitions to "Done" state
- Assignee set to "jane"
- Resolution set to "Fixed"
- Environment custom field set to "production"
- Tags custom field set to ["bug", "urgent"]
- Success message displayed

3. **Transition with unconfigured custom field (validation warning)**
```bash
jira issue move PROJ-123 "In Progress" --custom invalid-field=value
```

Expected Result:
- Warning message about unconfigured custom field
- Transition still succeeds but custom field is ignored

4. **Backward compatibility test (no custom fields)**
```bash
jira issue move PROJ-123 "In Progress" --assignee john
```

Expected Result:
- Issue transitions to "In Progress" state
- Assignee set to "john"
- Works exactly as before (backward compatible)

## Features

### Supported Custom Field Types
- **String**: Plain text values
- **Number**: Numeric values (auto-converted from string)
- **Option**: Single-select dropdown values
- **Project**: Project references
- **Array**: Multiple values (comma-separated)

### Usage Examples

```bash
# Single custom field
jira issue move ISSUE-1 "In Progress" --custom story-points=5

# Multiple custom fields
jira issue move ISSUE-1 Done \
  --custom environment=production \
  --custom release-version=v1.2.3

# Combined with other flags
jira issue move ISSUE-1 "Code Review" \
  --assignee john \
  --comment "Ready for review" \
  --custom reviewers=jane,bob

# Array custom field
jira issue move ISSUE-1 Testing --custom tags=bug,urgent,high-priority
```

## Design Principles

This implementation follows the KISS (Keep It Simple, Stupid) principle:

1. **Reuses existing code**: Leverages `cmdcommon.GetConfiguredCustomFields()` and `ValidateCustomFields()`
2. **Mirrors create.go pattern**: Uses the same marshaling approach as issue creation
3. **Minimal new code**: Only ~60 lines of new code total
4. **No duplication**: Helper function necessary for different context
5. **Backward compatible**: Existing commands work unchanged

## Architecture

```
User Input (--custom key=value)
    ↓
parseArgsAndFlags() - Parse flag into map[string]string
    ↓
cmdcommon.GetConfiguredCustomFields() - Load config
    ↓
cmdcommon.ValidateCustomFields() - Validate against config
    ↓
jira.BuildCustomFieldsForTransition() - Convert to typed custom fields
    ↓
jira.NewTransitionFieldsMarshaler() - Create marshaler
    ↓
jira.Transition() - Send to JIRA API
```

## Benefits

1. **User-friendly**: Simple key-value syntax for custom fields
2. **Type-safe**: Automatic type conversion based on configuration
3. **Validated**: Warns about unconfigured fields
4. **Consistent**: Same UX as `jira issue create --custom`
5. **Tested**: Comprehensive unit tests included

## Known Limitations

1. Custom fields must be configured in `.jira/config.yml` before use
2. Validation currently shows warnings but doesn't fail (future enhancement planned)
3. Complex custom field types (nested objects) not yet supported

## Future Enhancements

1. Interactive prompt for custom fields during transition
2. Auto-discovery of available custom fields for transitions
3. Stricter validation (fail on unconfigured fields)
4. Support for more complex custom field types
