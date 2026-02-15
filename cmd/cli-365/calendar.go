package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/lc0rp/cli-365/internal/owa"
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

					result, err := owa.ListCalendarEvents(
						client.Page(),
						client.Tokens(),
						startTime.Format(time.RFC3339),
						endTime.Format(time.RFC3339),
						c.Int("limit"),
						c.String("folder"),
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
