package cmd

import (
	"bufio"
	"cmp"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	hyperp "github.com/cloudwithax/swarmy/internal/agent/hyper"
	"github.com/cloudwithax/swarmy/internal/config"
	"github.com/cloudwithax/swarmy/internal/oauth"
	anthropicauth "github.com/cloudwithax/swarmy/internal/oauth/anthropic"
	"github.com/cloudwithax/swarmy/internal/oauth/codex"
	"github.com/cloudwithax/swarmy/internal/oauth/copilot"
	"github.com/cloudwithax/swarmy/internal/oauth/gitlab"
	"github.com/cloudwithax/swarmy/internal/oauth/hyper"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Aliases: []string{"auth"},
	Use:     "login [platform]",
	Short:   "Login Swarmy to a platform",
	Long: `Login Swarmy to a specified platform.
The platform should be provided as an argument.
Available platforms are: hyper, copilot, anthropic, codex, gitlab.`,
	Example: `
# Authenticate with Charm Hyper
swarmy login

# Authenticate with GitHub Copilot
swarmy login copilot

# Authenticate with Anthropic Claude Max
swarmy login anthropic

# Authenticate with OpenAI Codex
swarmy login codex

# Authenticate with GitLab
swarmy login gitlab
  `,
	ValidArgs: []cobra.Completion{
		"hyper",
		"copilot",
		"github",
		"github-copilot",
		"anthropic",
		"codex",
		"openai",
		"gitlab",
	},
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := setupAppWithProgressBar(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		provider := "hyper"
		if len(args) > 0 {
			provider = args[0]
		}
		switch provider {
		case "hyper":
			return loginHyper(app.Config())
		case "copilot", "github", "github-copilot":
			return loginCopilot(app.Config())
		case "anthropic":
			return loginAnthropic(app.Config())
		case "codex", "openai":
			return loginCodex(app.Config())
		case "gitlab":
			return loginGitlab(app.Config())
		default:
			return fmt.Errorf("unknown platform: %s", args[0])
		}
	},
}

func loginHyper(cfg *config.Config) error {
	if !hyperp.Enabled() {
		return fmt.Errorf("hyper not enabled")
	}
	ctx := getLoginContext()

	resp, err := hyper.InitiateDeviceAuth(ctx)
	if err != nil {
		return err
	}

	if clipboard.WriteAll(resp.UserCode) == nil {
		fmt.Println("The following code should be on clipboard already:")
	} else {
		fmt.Println("Copy the following code:")
	}

	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Bold(true).Render(resp.UserCode))
	fmt.Println()
	fmt.Println("Press enter to open this URL, and then paste it there:")
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Hyperlink(resp.VerificationURL, "id=hyper").Render(resp.VerificationURL))
	fmt.Println()
	waitEnter()
	if err := browser.OpenURL(resp.VerificationURL); err != nil {
		fmt.Println("Could not open the URL. You'll need to manually open the URL in your browser.")
	}

	fmt.Println("Exchanging authorization code...")
	refreshToken, err := hyper.PollForToken(ctx, resp.DeviceCode, resp.ExpiresIn)
	if err != nil {
		return err
	}

	fmt.Println("Exchanging refresh token for access token...")
	token, err := hyper.ExchangeToken(ctx, refreshToken)
	if err != nil {
		return err
	}

	fmt.Println("Verifying access token...")
	introspect, err := hyper.IntrospectToken(ctx, token.AccessToken)
	if err != nil {
		return fmt.Errorf("token introspection failed: %w", err)
	}
	if !introspect.Active {
		return fmt.Errorf("access token is not active")
	}

	if err := cmp.Or(
		cfg.SetConfigField("providers.hyper.api_key", token.AccessToken),
		cfg.SetConfigField("providers.hyper.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with Hyper!")
	return nil
}

func loginCopilot(cfg *config.Config) error {
	ctx := getLoginContext()

	if cfg.HasConfigField("providers.copilot.oauth") {
		fmt.Println("You are already logged in to GitHub Copilot.")
		return nil
	}

	diskToken, hasDiskToken := copilot.RefreshTokenFromDisk()
	var token *oauth.Token

	switch {
	case hasDiskToken:
		fmt.Println("Found existing GitHub Copilot token on disk. Using it to authenticate...")

		t, err := copilot.RefreshToken(ctx, diskToken)
		if err != nil {
			return fmt.Errorf("unable to refresh token from disk: %w", err)
		}
		token = t
	default:
		fmt.Println("Requesting device code from GitHub...")
		dc, err := copilot.RequestDeviceCode(ctx)
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("Open the following URL and follow the instructions to authenticate with GitHub Copilot:")
		fmt.Println()
		fmt.Println(lipgloss.NewStyle().Hyperlink(dc.VerificationURI, "id=copilot").Render(dc.VerificationURI))
		fmt.Println()
		fmt.Println("Code:", lipgloss.NewStyle().Bold(true).Render(dc.UserCode))
		fmt.Println()
		fmt.Println("Waiting for authorization...")

		t, err := copilot.PollForToken(ctx, dc)
		if err == copilot.ErrNotAvailable {
			fmt.Println()
			fmt.Println("GitHub Copilot is unavailable for this account. To signup, go to the following page:")
			fmt.Println()
			fmt.Println(lipgloss.NewStyle().Hyperlink(copilot.SignupURL, "id=copilot-signup").Render(copilot.SignupURL))
			fmt.Println()
			fmt.Println("You may be able to request free access if eligible. For more information, see:")
			fmt.Println()
			fmt.Println(lipgloss.NewStyle().Hyperlink(copilot.FreeURL, "id=copilot-free").Render(copilot.FreeURL))
		}
		if err != nil {
			return err
		}
		token = t
	}

	if err := cmp.Or(
		cfg.SetConfigField("providers.copilot.api_key", token.AccessToken),
		cfg.SetConfigField("providers.copilot.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with GitHub Copilot!")
	return nil
}

func getLoginContext() context.Context {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	go func() {
		<-ctx.Done()
		cancel()
		os.Exit(1)
	}()
	return ctx
}

func waitEnter() {
	_, _ = fmt.Scanln()
}

func loginAnthropic(cfg *config.Config) error {
	ctx := getLoginContext()

	if cfg.HasConfigField("providers.anthropic.oauth") {
		fmt.Println("You are already logged in to Anthropic.")
		return nil
	}

	authResp, err := anthropicauth.Authorize()
	if err != nil {
		return fmt.Errorf("failed to generate authorization URL: %w", err)
	}

	fmt.Println("Open the following URL to authorize Swarmy with Anthropic Claude:")
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Hyperlink(authResp.URL, "id=anthropic").Render(authResp.URL))
	fmt.Println()
	fmt.Println("After authorizing, you'll see a code on the page. Paste it here:")
	fmt.Print("> ")

	reader := bufio.NewReader(os.Stdin)
	combinedCode, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read code: %w", err)
	}
	combinedCode = strings.TrimSpace(combinedCode)
	if combinedCode == "" {
		return fmt.Errorf("no code entered")
	}

	fmt.Println("Exchanging code for tokens...")
	token, err := anthropicauth.Exchange(ctx, combinedCode, authResp.Verifier)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	if err := cmp.Or(
		cfg.SetConfigField("providers.anthropic.api_key", token.AccessToken),
		cfg.SetConfigField("providers.anthropic.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with Anthropic Claude!")
	return nil
}

func loginCodex(cfg *config.Config) error {
	ctx := getLoginContext()

	if cfg.HasConfigField("providers.openai.oauth") {
		fmt.Println("You are already logged in to OpenAI Codex.")
		return nil
	}

	fmt.Println("Requesting device code from OpenAI...")
	dc, err := codex.RequestDeviceCode(ctx)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Open the following URL and enter the code to authorize Swarmy with OpenAI Codex:")
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Hyperlink(dc.LoginURL, "id=codex").Render(dc.LoginURL))
	fmt.Println()
	fmt.Println("Code:", lipgloss.NewStyle().Bold(true).Render(dc.UserCode))
	fmt.Println()
	fmt.Println("Waiting for authorization...")

	token, err := codex.PollForToken(ctx, dc)
	if err != nil {
		return err
	}

	if err := cmp.Or(
		cfg.SetConfigField("providers.openai.api_key", token.AccessToken),
		cfg.SetConfigField("providers.openai.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with OpenAI Codex!")
	return nil
}

func loginGitlab(cfg *config.Config) error {
	ctx := getLoginContext()

	if cfg.HasConfigField("providers.gitlab.oauth") {
		fmt.Println("You are already logged in to GitLab.")
		return nil
	}

	instanceURL := os.Getenv("GITLAB_INSTANCE_URL")

	fmt.Println("Starting GitLab OAuth flow...")
	ba, err := gitlab.StartBrowserAuth(ctx, instanceURL)
	if err != nil {
		return fmt.Errorf("failed to start GitLab OAuth: %w", err)
	}

	fmt.Println()
	fmt.Println("Press enter to open this URL in your browser, or open it manually:")
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Hyperlink(ba.URL, "id=gitlab").Render(ba.URL))
	fmt.Println()
	waitEnter()
	if err := browser.OpenURL(ba.URL); err != nil {
		fmt.Println("Could not open the URL automatically. Please open it manually.")
	}

	fmt.Println("Waiting for authorization callback...")
	token, err := ba.Wait(ctx)
	if err != nil {
		return err
	}

	if err := cmp.Or(
		cfg.SetConfigField("providers.gitlab.api_key", token.AccessToken),
		cfg.SetConfigField("providers.gitlab.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with GitLab!")
	return nil
}
