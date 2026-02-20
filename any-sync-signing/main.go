package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/anyproto/any-sync/util/crypto"
	"github.com/mr-tron/base58"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "any-sync-signing",
		Short: "Derive Anytype identity and sign data",
		Long:  "Derive Ed25519 identity from a BIP39 mnemonic using SLIP-0010 and sign arbitrary data.",
		RunE:  runSign,
	}

	rootCmd.Flags().String("path", "", "derivation path (default: m/44'/2046' (anytype))")
	rootCmd.Flags().Uint32("index", 0, "account index")
	rootCmd.Flags().Bool("show-private", false, "also print the private key")

	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify a signature",
		Long:  "Verify a base58 signature against a message and an Anytype account.",
		RunE:  runVerify,
	}

	rootCmd.AddCommand(verifyCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runSign(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Read mnemonic securely
	fmt.Print("Enter BIP39 mnemonic (input is hidden): ")
	mnemonicBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // newline after hidden input
	if err != nil {
		return fmt.Errorf("reading mnemonic: %w", err)
	}
	mnemonic := crypto.Mnemonic(strings.TrimSpace(string(mnemonicBytes)))

	index, _ := cmd.Flags().GetUint32("index")

	// Derive keys
	res, err := mnemonic.DeriveKeys(index)
	if err != nil {
		return fmt.Errorf("key derivation failed: %w", err)
	}

	identity := res.Identity
	pub := identity.GetPublic()

	// Show account info
	pubRaw, _ := pub.Raw()
	fmt.Printf("\n--- Account %d ---\n", index)
	fmt.Printf("\n--- Public key ---\n")
	fmt.Printf("Account:    %s\n", pub.Account())
	fmt.Printf("PeerId:     %s\n", pub.PeerId())
	fmt.Printf("Raw:        %x\n", pubRaw)

	showPrivate, _ := cmd.Flags().GetBool("show-private")
	if showPrivate {
		privRaw, _ := identity.Raw()
		fmt.Printf("\n--- Private key ---\n")
		fmt.Printf("Raw: %x\n", privRaw)
	}

	// Interactive signing loop
	fmt.Println("\nEnter text to sign (empty line or Ctrl+D to exit):")
	for {
		fmt.Print("\n> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF
			fmt.Println()
			break
		}
		message := strings.TrimSpace(line)
		if message == "" {
			break
		}

		sig, err := identity.Sign([]byte(message))
		if err != nil {
			fmt.Fprintf(os.Stderr, "signing error: %v\n", err)
			continue
		}
		fmt.Printf("Signature (base58): %s\n", base58.Encode(sig))
	}

	return nil
}

func runVerify(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Read account
	fmt.Print("Account: ")
	accountLine, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading account: %w", err)
	}
	account := strings.TrimSpace(accountLine)

	// Decode public key from account string
	pub, err := crypto.DecodeAccountAddress(account)
	if err != nil {
		return fmt.Errorf("invalid account: %w", err)
	}

	// Read message
	fmt.Print("Message: ")
	messageLine, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading message: %w", err)
	}
	message := strings.TrimSpace(messageLine)

	// Read signature
	fmt.Print("Signature (base58): ")
	sigLine, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}
	sigB58 := strings.TrimSpace(sigLine)

	sig, err := base58.Decode(sigB58)
	if err != nil {
		return fmt.Errorf("invalid base58 signature: %w", err)
	}

	ok, err := pub.Verify([]byte(message), sig)
	if err != nil {
		return fmt.Errorf("verification error: %w", err)
	}

	if ok {
		fmt.Println("\nSignature is VALID")
	} else {
		fmt.Println("\nSignature is INVALID")
		os.Exit(1)
	}

	return nil
}
