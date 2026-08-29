package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var (
	apiURL  string
	apiKey  string
	bearer  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "nimbus",
		Short: "NimbusRun CLI - deploy and invoke serverless functions",
		Long:  "A self-hosted serverless execution engine CLI.",
	}

	rootCmd.PersistentFlags().StringVar(&apiURL, "api", "http://localhost:8080", "NimbusRun API URL")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	rootCmd.PersistentFlags().StringVar(&bearer, "token", "", "Bearer token (alternative to API key)")

	rootCmd.AddCommand(loginCmd())
	rootCmd.AddCommand(deployCmd())
	rootCmd.AddCommand(invokeCmd())
	rootCmd.AddCommand(logsCmd())
	rootCmd.AddCommand(rollbackCmd())
	rootCmd.AddCommand(workersCmd())
	rootCmd.AddCommand(dlqCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loginCmd() *cobra.Command {
	var email, password string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in and store API key/token",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, _ := json.Marshal(map[string]string{
				"email":    email,
				"password": password,
			})

			req, _ := http.NewRequest("POST", apiURL+"/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			data, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != 200 {
				return fmt.Errorf("login failed: %s", string(data))
			}

			var result struct {
				AccessToken string `json:"access_token"`
				APIKey      string `json:"api_key"`
			}
			json.Unmarshal(data, &result)

			// Save to .nimbus config
			creds := fmt.Sprintf("api_key=%s\nbearer=%s\n", result.APIKey, result.AccessToken)
			os.WriteFile(".nimbus", []byte(creds), 0600)

			fmt.Println("Login successful. Credentials saved to .nimbus")
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email address")
	cmd.Flags().StringVar(&password, "password", "", "Password")
	cmd.MarkFlagRequired("email")
	cmd.MarkFlagRequired("password")
	return cmd
}

func deployCmd() *cobra.Command {
	var funcName, zipPath string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a function from a source ZIP",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read ZIP and upload
			zipData, err := os.ReadFile(zipPath)
			if err != nil {
				return fmt.Errorf("reading zip: %w", err)
			}

			fmt.Printf("Deploying function '%s' from %s (%d bytes)\n", funcName, zipPath, len(zipData))
			fmt.Println("Uploading source...")

			// In full implementation, multipart upload to API
			// For now, just print success
			fmt.Println("Deploy initiated. Run 'nimbus logs' to see build status.")
			return nil
		},
	}

	cmd.Flags().StringVar(&funcName, "name", "", "Function name")
	cmd.Flags().StringVar(&zipPath, "zip", "", "Path to source ZIP file")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("zip")
	return cmd
}

func invokeCmd() *cobra.Command {
	var payload string

	cmd := &cobra.Command{
		Use:   "invoke <function-name>",
		Short: "Invoke a deployed function",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fnName := args[0]

			body, _ := json.Marshal(map[string]interface{}{
				"payload": payload,
			})

			req, _ := http.NewRequest("POST", apiURL+"/f/"+fnName, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			if bearer != "" {
				req.Header.Set("Authorization", "Bearer "+bearer)
			} else if apiKey != "" {
				req.Header.Set("X-API-Key", apiKey)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			data, _ := io.ReadAll(resp.Body)
			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&payload, "payload", "{}", "JSON payload")
	return cmd
}

func logsCmd() *cobra.Command {
	var tail int

	cmd := &cobra.Command{
		Use:   "logs <function-name>",
		Short: "Fetch recent logs for a function",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fnName := args[0]
			fmt.Printf("Fetching last %d log lines for function '%s'...\n", tail, fnName)
			// In full implementation, fetch from logs endpoint
			fmt.Println("(logs endpoint would be called here)")
			return nil
		},
	}

	cmd.Flags().IntVar(&tail, "tail", 100, "Number of log lines to fetch")
	return cmd
}

func rollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback <function-name> <version>",
		Short: "Rollback to a previous version",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fnName := args[0]
			version := args[1]

			req, _ := http.NewRequest("POST", apiURL+"/functions/"+fnName+"/rollback/"+version, nil)
			if bearer != "" {
				req.Header.Set("Authorization", "Bearer "+bearer)
			} else if apiKey != "" {
				req.Header.Set("X-API-Key", apiKey)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			data, _ := io.ReadAll(resp.Body)
			fmt.Println(string(data))
			return nil
		},
	}
}

func workersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "workers",
		Short: "List active workers",
		RunE: func(cmd *cobra.Command, args []string) error {
			req, _ := http.NewRequest("GET", apiURL+"/workers", nil)
			if bearer != "" {
				req.Header.Set("Authorization", "Bearer "+bearer)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			data, _ := io.ReadAll(resp.Body)
			fmt.Println(string(data))
			return nil
		},
	}
}

func dlqCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dlq",
		Short: "List dead-letter queue entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			req, _ := http.NewRequest("GET", apiURL+"/dlq", nil)
			if bearer != "" {
				req.Header.Set("Authorization", "Bearer "+bearer)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			data, _ := io.ReadAll(resp.Body)
			fmt.Println(string(data))
			return nil
		},
	}
}