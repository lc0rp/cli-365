package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/lc0rp/cli-365/internal/owa"
	"github.com/lc0rp/cli-365/internal/paths"
)

func calendarCommand() *cli.Command {
	return &cli.Command{
		Name:  "calendar",
		Usage: "Calendar event operations",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List calendar events",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "start", Usage: "Start date/time (YYYY-MM-DD or RFC3339)"},
					&cli.StringFlag{Name: "end", Usage: "End date/time (YYYY-MM-DD or RFC3339)"},
					&cli.IntFlag{Name: "limit", Aliases: []string{"n"}, Value: 50, Usage: "Maximum events to return"},
					&cli.StringFlag{Name: "folder", Usage: "Calendar folder ID (default: calendar)"},
					&cli.StringFlag{Name: "calendar", Usage: "Calendar selector: calendar_id, name, or email"},
				},
				Action: func(c *cli.Context) error {
					client, err := getOWAClient(c)
					if err != nil {
						return err
					}

					startTime, endTime, err := resolveCalendarRange(c.String("start"), c.String("end"))
					if err != nil {
						return err
					}

					folderID := strings.TrimSpace(c.String("folder"))
					calendarSelector := strings.TrimSpace(c.String("calendar"))
					if folderID != "" && calendarSelector != "" {
						return cli.Exit("--folder and --calendar cannot both be set", 1)
					}
					if calendarSelector != "" {
						folders, err := owa.ListCalendarFolders(client.Page(), client.Tokens())
						if err != nil {
							return err
						}
						registry, err := loadAddedDirectoryCalendars()
						if err != nil {
							return err
						}
						items := mergeCalendarListItems(folders, registry)
						resolved, err := resolveCalendarListFolderFromSelector(items, calendarSelector)
						if err != nil {
							return cli.Exit(err.Error(), 1)
						}
						folderID = resolved
					}

					result, err := owa.ListCalendarEvents(
						client.Page(),
						client.Tokens(),
						startTime.Format(time.RFC3339),
						endTime.Format(time.RFC3339),
						c.Int("limit"),
						folderID,
					)
					if err != nil {
						return err
					}

					if c.Bool("json") {
						return outputJSON(result)
					}

					if result == nil || len(result.Events) == 0 {
						fmt.Println("No events found")
						return nil
					}

					for i, ev := range result.Events {
						subject := strings.TrimSpace(ev.Subject)
						if subject == "" {
							subject = "(no subject)"
						}
						fmt.Printf("%d %s %s - %s\n", i+1, ev.ID, formatCalendarRange(ev.Start, ev.End), subject)
					}
					return nil
				},
			},
			{
				Name:    "calendars",
				Aliases: []string{"folders"},
				Usage:   "List calendars with known IDs and metadata",
				Action: func(c *cli.Context) error {
					client, err := getOWAClient(c)
					if err != nil {
						return err
					}
					folders, err := owa.ListCalendarFolders(client.Page(), client.Tokens())
					if err != nil {
						return err
					}
					registry, err := loadAddedDirectoryCalendars()
					if err != nil {
						return err
					}
					items := mergeCalendarListItems(folders, registry)
					if c.Bool("json") {
						return outputJSON(items)
					}
					if len(items) == 0 {
						fmt.Println("No calendars found")
						return nil
					}
					for i, item := range items {
						name := item.Name
						if strings.TrimSpace(name) == "" {
							name = "(no name)"
						}
						fmt.Printf("%d %s\n", i+1, name)
						fmt.Printf("  Folder ID: %s\n", item.FolderID)
						if item.CalendarID != "" {
							fmt.Printf("  Calendar ID: %s\n", item.CalendarID)
						}
						if item.Email != "" {
							fmt.Printf("  Email: %s\n", item.Email)
						}
					}
					return nil
				},
			},
			{
				Name:      "get",
				Usage:     "Get a calendar event",
				ArgsUsage: "<event-id>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return cli.Exit("event ID required", 1)
					}
					client, err := getOWAClient(c)
					if err != nil {
						return err
					}
					eventID := c.Args().First()
					event, err := owa.GetCalendarEvent(client.Page(), client.Tokens(), eventID)
					if err != nil {
						return err
					}

					if c.Bool("json") {
						return outputJSON(event)
					}

					fmt.Printf("Subject: %s\n", event.Subject)
					fmt.Printf("Start: %s\n", event.Start)
					fmt.Printf("End: %s\n", event.End)
					if event.Location != nil && strings.TrimSpace(event.Location.DisplayName) != "" {
						fmt.Printf("Location: %s\n", event.Location.DisplayName)
					}
					if event.Organizer != nil && event.Organizer.Mailbox.Address != "" {
						fmt.Printf("Organizer: %s\n", formatEmailAddress(event.Organizer.Mailbox))
					}
					if attendees := formatAttendees(event.RequiredAttendees); attendees != "" {
						fmt.Printf("Required: %s\n", attendees)
					}
					if attendees := formatAttendees(event.OptionalAttendees); attendees != "" {
						fmt.Printf("Optional: %s\n", attendees)
					}
					if event.Body != nil {
						fmt.Println()
						fmt.Println(event.Body.Value)
					}
					return nil
				},
			},
			{
				Name:      "add-from-directory",
				Aliases:   []string{"add-directory"},
				Usage:     "Add a colleague/resource calendar from directory",
				ArgsUsage: "[email-or-name]",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "email", Usage: "Directory email address"},
					&cli.StringFlag{Name: "name", Usage: "Directory display name"},
					&cli.StringFlag{Name: "display-name", Usage: "Calendar label to use locally"},
					&cli.BoolFlag{Name: "allow-ambiguous", Usage: "Pick top directory result when name matches multiple people"},
				},
				Action: func(c *cli.Context) error {
					email, name, err := resolveCalendarDirectoryIdentity(
						c.String("email"),
						c.String("name"),
						c.Args().First(),
					)
					if err != nil {
						return cli.Exit(err.Error(), 1)
					}

					client, err := getOWAClient(c)
					if err != nil {
						return err
					}
					folders, err := owa.ListCalendarFolders(client.Page(), client.Tokens())
					if err != nil {
						return err
					}
					registry, err := loadAddedDirectoryCalendars()
					if err != nil {
						return err
					}
					existing, ok := findExistingAddedDirectoryCalendar(registry, folders, email, name)
					if ok {
						result := &owa.DirectoryCalendarResult{
							Email:           existing.Email,
							ResolvedName:    existing.ResolvedName,
							CalendarName:    existing.CalendarName,
							FolderID:        existing.FolderID,
							CalendarID:      existing.CalendarID,
							CalendarGroupID: existing.CalendarGroupID,
							AlreadyExists:   true,
						}
						if c.Bool("json") {
							return outputJSON(result)
						}
						display := result.CalendarName
						if display == "" {
							display = result.ResolvedName
						}
						if strings.TrimSpace(display) == "" {
							display = result.Email
						}
						fmt.Printf("Calendar already added: %s\n", formatCalendarDirectoryDisplay(display, result.Email))
						fmt.Printf("Folder ID: %s\n", result.FolderID)
						if strings.TrimSpace(result.CalendarID) != "" {
							fmt.Printf("Calendar ID: %s\n", result.CalendarID)
						}
						return nil
					}

					result, err := owa.AddDirectoryCalendar(client.Page(), client.Tokens(), owa.DirectoryCalendarInput{
						Email:          email,
						Name:           name,
						DisplayName:    c.String("display-name"),
						AllowAmbiguous: c.Bool("allow-ambiguous"),
					})
					if err != nil {
						return err
					}
					registry = upsertAddedDirectoryCalendar(registry, addedDirectoryCalendarRecord{
						Email:           result.Email,
						ResolvedName:    result.ResolvedName,
						CalendarName:    result.CalendarName,
						FolderID:        result.FolderID,
						CalendarID:      result.CalendarID,
						CalendarGroupID: result.CalendarGroupID,
						AddedAt:         time.Now().UTC(),
					})
					if err := saveAddedDirectoryCalendars(registry); err != nil {
						return err
					}
					if c.Bool("json") {
						return outputJSON(result)
					}

					display := result.CalendarName
					if strings.TrimSpace(display) == "" {
						display = result.Email
					}
					fmt.Printf("Calendar added: %s\n", formatCalendarDirectoryDisplay(display, result.Email))
					fmt.Printf("Folder ID: %s\n", result.FolderID)
					if strings.TrimSpace(result.CalendarID) != "" {
						fmt.Printf("Calendar ID: %s\n", result.CalendarID)
					}
					return nil
				},
			},
			{
				Name:  "create",
				Usage: "Create a calendar event",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "subject", Aliases: []string{"s"}, Required: true, Usage: "Event subject"},
					&cli.StringFlag{Name: "start", Required: true, Usage: "Start date/time (YYYY-MM-DD or RFC3339)"},
					&cli.StringFlag{Name: "end", Required: true, Usage: "End date/time (YYYY-MM-DD or RFC3339)"},
					&cli.StringFlag{Name: "location", Usage: "Event location"},
					&cli.BoolFlag{Name: "all-day", Usage: "Mark event as all-day"},
					&cli.StringFlag{Name: "body", Aliases: []string{"b"}, Usage: "Event body"},
					&cli.StringFlag{Name: "body-type", Value: "Text", Usage: "Body type (Text or HTML)"},
					&cli.StringSliceFlag{Name: "attendee", Usage: "Required attendee email (repeatable)"},
					&cli.StringSliceFlag{Name: "optional-attendee", Usage: "Optional attendee email (repeatable)"},
				},
				Action: func(c *cli.Context) error {
					client, err := getOWAClient(c)
					if err != nil {
						return err
					}

					start, err := parseCalendarTime(c.String("start"))
					if err != nil {
						return err
					}
					end, err := parseCalendarTime(c.String("end"))
					if err != nil {
						return err
					}

					draft := &owa.CalendarEventDraft{
						Subject:       c.String("subject"),
						Start:         start.Format(time.RFC3339),
						End:           end.Format(time.RFC3339),
						IsAllDayEvent: c.Bool("all-day"),
						Location:      c.String("location"),
					}

					if body := c.String("body"); body != "" {
						draft.Body = &owa.MessageBody{BodyType: c.String("body-type"), Value: body}
					}

					draft.RequiredAttendees = parseAttendees(c.StringSlice("attendee"))
					draft.OptionalAttendees = parseAttendees(c.StringSlice("optional-attendee"))

					event, err := owa.CreateCalendarEvent(client.Page(), client.Tokens(), draft)
					if err != nil {
						return err
					}

					if c.Bool("json") {
						return outputJSON(event)
					}

					fmt.Printf("Event created: %s\n", event.ID)
					return nil
				},
			},
			{
				Name:      "update",
				Usage:     "Update a calendar event",
				ArgsUsage: "<event-id>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "subject", Aliases: []string{"s"}, Usage: "Event subject"},
					&cli.StringFlag{Name: "start", Usage: "Start date/time (YYYY-MM-DD or RFC3339)"},
					&cli.StringFlag{Name: "end", Usage: "End date/time (YYYY-MM-DD or RFC3339)"},
					&cli.StringFlag{Name: "location", Usage: "Event location"},
					&cli.BoolFlag{Name: "all-day", Usage: "Mark event as all-day"},
					&cli.BoolFlag{Name: "timed", Usage: "Mark event as timed"},
					&cli.StringFlag{Name: "body", Aliases: []string{"b"}, Usage: "Event body"},
					&cli.StringFlag{Name: "body-type", Value: "Text", Usage: "Body type (Text or HTML)"},
				},
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return cli.Exit("event ID required", 1)
					}
					if c.Bool("all-day") && c.Bool("timed") {
						return cli.Exit("--all-day and --timed cannot both be set", 1)
					}
					client, err := getOWAClient(c)
					if err != nil {
						return err
					}

					update := &owa.CalendarEventUpdate{}
					if c.IsSet("subject") {
						value := c.String("subject")
						update.Subject = &value
					}
					if c.IsSet("start") {
						t, err := parseCalendarTime(c.String("start"))
						if err != nil {
							return err
						}
						value := t.Format(time.RFC3339)
						update.Start = &value
					}
					if c.IsSet("end") {
						t, err := parseCalendarTime(c.String("end"))
						if err != nil {
							return err
						}
						value := t.Format(time.RFC3339)
						update.End = &value
					}
					if c.IsSet("location") {
						value := c.String("location")
						update.Location = &value
					}
					if c.Bool("all-day") || c.Bool("timed") {
						value := c.Bool("all-day")
						update.IsAllDayEvent = &value
					}
					if c.IsSet("body") {
						update.Body = &owa.MessageBody{BodyType: c.String("body-type"), Value: c.String("body")}
					}

					eventID := c.Args().First()
					if err := owa.UpdateCalendarEvent(client.Page(), client.Tokens(), eventID, update); err != nil {
						return err
					}

					if c.Bool("json") {
						return outputJSON(map[string]string{"status": "updated"})
					}

					fmt.Println("Event updated")
					return nil
				},
			},
			{
				Name:      "delete",
				Usage:     "Delete a calendar event",
				ArgsUsage: "<event-id>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return cli.Exit("event ID required", 1)
					}
					client, err := getOWAClient(c)
					if err != nil {
						return err
					}
					eventID := c.Args().First()
					if err := owa.DeleteCalendarEvent(client.Page(), client.Tokens(), eventID); err != nil {
						return err
					}
					fmt.Println("Event deleted")
					return nil
				},
			},
		},
	}
}

func resolveCalendarRange(startRaw string, endRaw string) (time.Time, time.Time, error) {
	if startRaw == "" {
		start := time.Now()
		end := start.Add(7 * 24 * time.Hour)
		if endRaw != "" {
			parsedEnd, err := parseCalendarTime(endRaw)
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
			end = parsedEnd
		}
		return start, end, nil
	}
	start, err := parseCalendarTime(startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	var end time.Time
	if endRaw == "" {
		end = start.Add(7 * 24 * time.Hour)
	} else {
		end, err = parseCalendarTime(endRaw)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be after start")
	}
	return start, end, nil
}

func parseCalendarTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("date/time is required")
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", raw, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD or RFC3339)", raw)
}

func parseAttendees(values []string) []owa.EmailAddress {
	if len(values) == 0 {
		return nil
	}
	out := make([]owa.EmailAddress, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, owa.EmailAddress{Address: value})
	}
	return out
}

func formatAttendees(attendees []owa.Attendee) string {
	if len(attendees) == 0 {
		return ""
	}
	parts := make([]string, 0, len(attendees))
	for _, attendee := range attendees {
		if attendee.Mailbox.Address == "" {
			continue
		}
		parts = append(parts, formatEmailAddress(attendee.Mailbox))
	}
	return strings.Join(parts, ", ")
}

func formatEmailAddress(addr owa.EmailAddress) string {
	if addr.Address == "" {
		return ""
	}
	if addr.Name != "" {
		return fmt.Sprintf("%s <%s>", addr.Name, addr.Address)
	}
	return addr.Address
}

func formatCalendarRange(start string, end string) string {
	if start == "" && end == "" {
		return ""
	}
	if start == "" {
		return end
	}
	if end == "" {
		return start
	}
	return fmt.Sprintf("%s → %s", start, end)
}

func resolveCalendarDirectoryIdentity(emailFlag string, nameFlag string, arg string) (string, string, error) {
	email := strings.TrimSpace(emailFlag)
	name := strings.TrimSpace(nameFlag)
	arg = strings.TrimSpace(arg)
	if email != "" && name != "" {
		return "", "", fmt.Errorf("--email and --name cannot both be set")
	}
	if arg != "" && (email != "" || name != "") {
		return "", "", fmt.Errorf("positional identity cannot be combined with --email or --name")
	}

	if arg != "" {
		if strings.Contains(arg, "@") {
			email = arg
		} else {
			name = arg
		}
	}
	if email == "" && name == "" {
		return "", "", fmt.Errorf("provide --email, --name, or [email-or-name]")
	}
	if email != "" && !strings.Contains(email, "@") {
		return "", "", fmt.Errorf("invalid email %q", email)
	}
	return email, name, nil
}

type addedDirectoryCalendarRecord struct {
	Email           string    `json:"email"`
	ResolvedName    string    `json:"resolved_name,omitempty"`
	CalendarName    string    `json:"calendar_name,omitempty"`
	FolderID        string    `json:"folder_id"`
	CalendarID      string    `json:"calendar_id,omitempty"`
	CalendarGroupID string    `json:"calendar_group_id,omitempty"`
	AddedAt         time.Time `json:"added_at"`
}

type calendarListItem struct {
	Name       string `json:"name"`
	Email      string `json:"email,omitempty"`
	FolderID   string `json:"folder_id"`
	CalendarID string `json:"calendar_id,omitempty"`
}

func addedCalendarsPath() string {
	return filepath.Join(paths.StateDir(), "cli-365", "added_calendars.json")
}

func loadAddedDirectoryCalendars() ([]addedDirectoryCalendarRecord, error) {
	path := addedCalendarsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var records []addedDirectoryCalendarRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func saveAddedDirectoryCalendars(records []addedDirectoryCalendarRecord) error {
	path := addedCalendarsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func upsertAddedDirectoryCalendar(records []addedDirectoryCalendarRecord, record addedDirectoryCalendarRecord) []addedDirectoryCalendarRecord {
	for i := range records {
		if strings.EqualFold(strings.TrimSpace(records[i].Email), strings.TrimSpace(record.Email)) ||
			(strings.TrimSpace(records[i].FolderID) != "" && strings.TrimSpace(records[i].FolderID) == strings.TrimSpace(record.FolderID)) {
			records[i] = record
			return records
		}
	}
	return append(records, record)
}

func findExistingAddedDirectoryCalendar(
	records []addedDirectoryCalendarRecord,
	folders []owa.CalendarFolder,
	email string,
	name string,
) (addedDirectoryCalendarRecord, bool) {
	email = normalizeCalendarIdentity(email)
	name = normalizeCalendarIdentity(name)
	if email == "" && name == "" {
		return addedDirectoryCalendarRecord{}, false
	}
	activeFolders := make(map[string]struct{}, len(folders))
	folderNames := make(map[string]string, len(folders))
	for _, folder := range folders {
		id := strings.TrimSpace(folder.FolderID)
		if id == "" {
			continue
		}
		activeFolders[id] = struct{}{}
		folderName := strings.TrimSpace(folder.DisplayName)
		if folderName != "" {
			folderNames[id] = folderName
		}
	}

	for _, record := range records {
		if record.FolderID == "" {
			continue
		}
		_, isActiveFolder := activeFolders[record.FolderID]
		recordEmail := normalizeCalendarIdentity(record.Email)
		recordName := normalizeCalendarIdentity(record.ResolvedName)
		recordCalendarName := normalizeCalendarIdentity(record.CalendarName)
		recordFolderName := normalizeCalendarIdentity(folderNames[record.FolderID])
		if email != "" && recordEmail == email {
			return record, true
		}
		if name != "" && (recordName == name || recordCalendarName == name || (isActiveFolder && recordFolderName == name)) {
			return record, true
		}
	}

	for _, folder := range folders {
		folderName := normalizeCalendarIdentity(folder.DisplayName)
		if folderName == "" {
			continue
		}
		if (name != "" && folderName == name) || (email != "" && folderName == email) {
			return addedDirectoryCalendarRecord{
				Email:        strings.TrimSpace(email),
				ResolvedName: strings.TrimSpace(folder.DisplayName),
				CalendarName: strings.TrimSpace(folder.DisplayName),
				FolderID:     strings.TrimSpace(folder.FolderID),
			}, true
		}
	}

	return addedDirectoryCalendarRecord{}, false
}

func mergeCalendarListItems(folders []owa.CalendarFolder, records []addedDirectoryCalendarRecord) []calendarListItem {
	byFolder := make(map[string]addedDirectoryCalendarRecord, len(records))
	for _, record := range records {
		if id := strings.TrimSpace(record.FolderID); id != "" {
			byFolder[id] = record
		}
	}

	items := make([]calendarListItem, 0, len(folders))
	seenFolders := make(map[string]struct{}, len(folders))
	for _, folder := range folders {
		if id := strings.TrimSpace(folder.FolderID); id != "" {
			seenFolders[id] = struct{}{}
		}
		record, hasRecord := byFolder[strings.TrimSpace(folder.FolderID)]
		item := calendarListItem{
			Name:     folder.DisplayName,
			FolderID: folder.FolderID,
		}
		if hasRecord {
			if strings.TrimSpace(record.CalendarName) != "" {
				item.Name = record.CalendarName
			} else if strings.TrimSpace(record.ResolvedName) != "" {
				item.Name = record.ResolvedName
			}
			item.Email = record.Email
			item.CalendarID = record.CalendarID
		}
		items = append(items, item)
	}
	for _, record := range records {
		folderID := strings.TrimSpace(record.FolderID)
		if folderID == "" {
			continue
		}
		if _, ok := seenFolders[folderID]; ok {
			continue
		}
		name := strings.TrimSpace(record.CalendarName)
		if name == "" {
			name = strings.TrimSpace(record.ResolvedName)
		}
		if name == "" {
			name = strings.TrimSpace(record.Email)
		}
		items = append(items, calendarListItem{
			Name:       name,
			Email:      strings.TrimSpace(record.Email),
			FolderID:   folderID,
			CalendarID: strings.TrimSpace(record.CalendarID),
		})
		seenFolders[folderID] = struct{}{}
	}
	return items
}

func normalizeCalendarIdentity(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func formatCalendarDirectoryDisplay(display string, email string) string {
	display = strings.TrimSpace(display)
	email = strings.TrimSpace(email)
	switch {
	case display == "" && email == "":
		return "(unknown)"
	case display == "":
		return email
	case email == "":
		return display
	default:
		return fmt.Sprintf("%s <%s>", display, email)
	}
}

func resolveCalendarListFolderFromSelector(items []calendarListItem, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", fmt.Errorf("calendar selector is required")
	}

	findUnique := func(matches []calendarListItem, kind string) (string, error) {
		if len(matches) == 0 {
			return "", nil
		}
		if len(matches) == 1 {
			return strings.TrimSpace(matches[0].FolderID), nil
		}
		samples := make([]string, 0, len(matches))
		for _, match := range matches {
			name := strings.TrimSpace(match.Name)
			if name == "" {
				name = "(no name)"
			}
			samples = append(samples, fmt.Sprintf("%s [%s]", name, strings.TrimSpace(match.FolderID)))
			if len(samples) == 3 {
				break
			}
		}
		return "", fmt.Errorf("multiple calendars matched %s %q: %s", kind, selector, strings.Join(samples, ", "))
	}

	if strings.Contains(selector, "@") {
		matches := make([]calendarListItem, 0, 1)
		for _, item := range items {
			if strings.EqualFold(strings.TrimSpace(item.Email), selector) {
				matches = append(matches, item)
			}
		}
		if folderID, err := findUnique(matches, "email"); folderID != "" || err != nil {
			return folderID, err
		}
		return "", fmt.Errorf("no calendar matched email %q", selector)
	}

	if looksLikeCalendarOpaqueID(selector) {
		matches := make([]calendarListItem, 0, 1)
		for _, item := range items {
			if strings.TrimSpace(item.CalendarID) == selector || strings.TrimSpace(item.FolderID) == selector {
				matches = append(matches, item)
			}
		}
		if folderID, err := findUnique(matches, "id"); folderID != "" || err != nil {
			return folderID, err
		}
	}

	matches := make([]calendarListItem, 0, 1)
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Name), selector) {
			matches = append(matches, item)
		}
	}
	if folderID, err := findUnique(matches, "name"); folderID != "" || err != nil {
		return folderID, err
	}
	return "", fmt.Errorf("no calendar matched %q (use `cli-365 calendar calendars` to inspect names/ids)", selector)
}

func looksLikeCalendarOpaqueID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 20 {
		return false
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	return strings.HasPrefix(strings.ToUpper(value), "AA")
}
