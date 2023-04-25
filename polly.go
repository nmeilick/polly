package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/emersion/go-imap/client"
	"github.com/kballard/go-shellquote"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

const (
	DefaultMailbox       = "INBOX"
	DefaultCheckInterval = 30 * time.Second
)

var log zerolog.Logger

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	var out io.Writer = os.Stderr
	color := isatty.IsTerminal(os.Stderr.Fd())
	if color {
		out = colorable.NewColorableStderr()
	}

	log = zerolog.New(zerolog.ConsoleWriter{
		Out:        out,
		TimeFormat: "2006-01-02 15:04:05",
		NoColor:    !color,
	}).With().Timestamp().Logger()

	app := &cli.App{
		Name:            filepath.Base(os.Args[0]),
		Version:         fullVersion(),
		Usage:           "Run a script when new mail is received",
		HideHelpCommand: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "server",
				Aliases: []string{"s"},
				EnvVars: []string{"IMAP_SERVER"},
				Usage:   "IMAP server address (e.g., imap.example.org:993)",
			},
			&cli.StringFlag{
				Name:    "username",
				Aliases: []string{"u"},
				EnvVars: []string{"IMAP_USERNAME"},
				Usage:   "IMAP username",
			},
			&cli.StringFlag{
				Name:    "password",
				Aliases: []string{"p"},
				EnvVars: []string{"IMAP_PASSWORD"},
				Usage:   "IMAP password",
			},
			&cli.StringFlag{
				Name:    "mailbox",
				Aliases: []string{"m"},
				EnvVars: []string{"IMAP_MAILBOX"},
				Usage:   "IMAP mailbox",
				Value:   DefaultMailbox,
			},
			&cli.StringFlag{
				Name:    "script",
				Aliases: []string{"S"},
				EnvVars: []string{"SCRIPT"},
				Usage:   "Script to run when new mail arrives",
			},
			&cli.DurationFlag{
				Name:    "check-interval",
				Aliases: []string{"i"},
				EnvVars: []string{"CHECK_INTERVAL"},
				Usage:   "If idle mode is not available, interval between mailbox checks (in seconds)",
				Value:   DefaultCheckInterval,
			},
		},
		Action: run,
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func fail(format string, args ...interface{}) {
	if len(args) > 0 {
		format = fmt.Sprintf(format, args...)
	}
	fmt.Fprintln(os.Stderr, format)
	os.Exit(1)
}

func run(ctx *cli.Context) error {
	server := ctx.String("server")
	username := ctx.String("username")
	password := ctx.String("password")
	mailbox := ctx.String("mailbox")
	script := ctx.String("script")
	checkInterval := ctx.Duration("check-interval")
	args := ctx.Args().Slice()

	var command []string

	if script == "" {
		command = append(command, args...)
		args = nil
	} else {
		command = append(command, script)
	}

	switch {
	case server == "":
		fail("Please specify --server or set IMAP_SERVER in env.")
	case username == "":
		fail("Please specify --username or set IMAP_USERNAME in env.")
	case password == "":
		fail("Please specify --password or set IMAP_PASSWORD in env.")
	case mailbox == "":
		fail("Please specify --mailbox or set IMAP_MAILBOX in env.")
	case len(command) == 0:
		fail("Please specify --script or set SCRIPT in env.")
	case checkInterval.Seconds() < 5:
		fail("Error: the given --check-interval should be at least 5s.")
	case len(args) > 0:
		fail("Error: extraneous arguments: %s", shellquote.Join(args...))
	}

	var c *client.Client
	var err error
	for {
		if c != nil {
			c.Logout()
			time.Sleep(60 * time.Second)
		}

		log.Info().Msgf("Connecting to IMAP server: %s", server)
		if c, err = client.DialTLS(server, nil); err != nil {
			log.Err(err).Msg("Error connecting to IMAP server")
			time.Sleep(60 * time.Second)
			continue
		}
		if os.Getenv("DEBUG") == "1" {
			c.SetDebug(os.Stderr)
		}

		log.Info().Msgf("Logging in with user: %s", username)
		if err = c.Login(username, password); err != nil {
			log.Err(err).Msg("Error logging in")
			continue
		}

		log.Info().Msgf("Selecting mailbox: %s", mailbox)
		if _, err = c.Select(mailbox, false); err != nil {
			log.Err(err).Msg("Error selecting mailbox")
			continue
		}

		updates := make(chan client.Update, 32)
		c.Updates = updates
		stop := make(chan struct{}, 1)

		go func() {
			for {
				select {
				case u := <-updates:
					log.Info().Msgf("Received IMAP update: %T", u)
					if _, ok := u.(*client.MailboxUpdate); ok {
						notify(command)
					}
				case <-stop:
					return
				}
			}
		}()

		log.Info().Msg("Entering idle mode...")
		opts := &client.IdleOptions{
			LogoutTimeout: 15 * time.Minute,
			PollInterval:  checkInterval,
		}
		if err = c.Idle(stop, opts); err != nil {
			log.Err(err).Msg("Idling failed")
		}
		close(stop)
	}
}

func notify(command []string) {
	log.Info().Msg("New mail detected. Executing script...")
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		log.Error().Err(err).Msg("Error executing script")
	}
}
