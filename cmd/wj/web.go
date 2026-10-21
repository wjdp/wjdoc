package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/web"
)

func webCommand(shared *app) *cobra.Command {
	var address string
	var noOpen bool

	command := &cobra.Command{
		Use:   "web",
		Short: "Browse and edit the documents in a browser",
		Long: `Serve a viewer and editor for docs/ on localhost and open it in the browser.

The sidebar lists the documents, active ones by default; the status and type
selectors widen or narrow it, and the filter box matches number and title. The
pane on the right renders the selected document and edits its source in place.

Changing the status rewrites only that frontmatter line, as wj set does. Saving
refuses when the file changed on disk since it was loaded. Changes made outside
the browser show up when its tab regains focus.

Keys: j and k move through the list, / filters, e edits, Ctrl+S saves and Esc
returns to the rendered view.

Examples:
  wj web                       serve on a free port and open the browser
  wj web --addr :7777          a fixed port, still bound to localhost only
  wj web --no-open             print the address without opening a browser`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			listener, err := net.Listen("tcp", loopbackAddress(address))
			if err != nil {
				return err
			}
			url := "http://" + listener.Addr().String() + "/"
			fmt.Fprintln(command.OutOrStdout(), "serving", shared.root, "at", url)

			if !noOpen {
				if err := openBrowser(url); err != nil {
					fmt.Fprintln(command.ErrOrStderr(), "wj: could not open a browser:", err)
				}
			}

			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt)
			defer stop()

			server := &http.Server{Handler: web.Handler(shared.root), ReadHeaderTimeout: 10 * time.Second}
			go func() {
				<-ctx.Done()
				shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = server.Shutdown(shutdown)
			}()

			if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}

	command.Flags().StringVar(&address, "addr", "127.0.0.1:0", "listen on `host:port`; a bare :port binds 127.0.0.1")
	command.Flags().BoolVar(&noOpen, "no-open", false, "do not open a browser")
	return command
}

// loopbackAddress binds a bare ":port" to 127.0.0.1 rather than every
// interface, since the editor writes to disk.
func loopbackAddress(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err == nil && host == "" {
		return net.JoinHostPort("127.0.0.1", port)
	}
	return address
}

func openBrowser(url string) error {
	name, args := "xdg-open", []string{url}
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	}
	return exec.Command(name, args...).Start()
}
