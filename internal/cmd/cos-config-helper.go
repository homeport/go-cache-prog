// Copyright © 2026 The Homeport Team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/homeport/go-cache-prog/internal/cosconfighelper"
	"github.com/homeport/go-cache-prog/pkg/provider/cos"
	"github.com/spf13/cobra"
)

type cosConfigHelperOpts struct {
	outputFile string
	verbose    bool
}

var cosConfigHelperSettings cosConfigHelperOpts

var cosConfigHelperCmd = &cobra.Command{
	Use:   "config-helper",
	Short: "Interactively generate a GO_CACHE_PROG_COS_CONFIG JSON blob",
	Long: `Interactively walk through your IBM Cloud account to select a COS instance,
HMAC credentials, and bucket, then output a ready-to-use JSON configuration
blob for the GO_CACHE_PROG_COS_CONFIG environment variable.

Requires an active IBM Cloud session (run 'ibmcloud login' first).`,
	SilenceUsage:  true,
	SilenceErrors: true,

	RunE: runCosConfigHelper,
}

func init() {
	cosCmd.AddCommand(cosConfigHelperCmd)
	cosConfigHelperCmd.Flags().StringVarP(&cosConfigHelperSettings.outputFile, "output", "o", "", "write JSON to this file instead of stdout")
	cosConfigHelperCmd.Flags().BoolVarP(&cosConfigHelperSettings.verbose, "verbose", "v", false, "trace HTTP requests and responses to stderr")
}

func runCosConfigHelper(cmd *cobra.Command, _ []string) error {
	// Open /dev/tty directly so huh's raw terminal I/O is fully isolated from
	// os.Stdin and os.Stdout. This prevents bubbletea from replaying buffered
	// input back into cobra after the form exits.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("cannot open terminal: %w", err)
	}
	defer func() { _ = tty.Close() }()

	stderr := tty

	h := cosconfighelper.New(cosConfigHelperSettings.verbose)

	// ── Step 1: Read IBM Cloud local session ────────────────────────────────
	ibmCfg, err := cosconfighelper.ReadIBMCloudConfig()
	if err != nil {
		return fmt.Errorf("no active IBM Cloud session found — please run 'ibmcloud login' first\n(%w)", err)
	}
	if ibmCfg.IAMToken == "" {
		return fmt.Errorf("no active IBM Cloud session found — please run 'ibmcloud login' first")
	}
	if expiry, err := cosconfighelper.IAMTokenExpiry(ibmCfg.IAMToken); err == nil && time.Now().After(expiry) {
		return fmt.Errorf("IBM Cloud session expired — please run 'ibmcloud login' first (expired %s ago)",
			time.Since(expiry).Truncate(time.Minute))
	}

	h.IBMCfg = ibmCfg

	// ── Step 2: Confirm account ──────────────────────────────────────────────
	accountLabel := ibmCfg.AccountName
	if accountLabel == "" {
		accountLabel = "(unnamed account)"
	}

	accountConfirmed := true
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Active IBM Cloud account: %s", accountLabel)).
				Description("Is this the correct account?").
				Value(&accountConfirmed),
		),
	).WithInput(tty).WithOutput(tty).Run(); err != nil {
		return err
	}
	if !accountConfirmed {
		_, _ = fmt.Fprintln(stderr, "Please run 'ibmcloud login' to switch accounts, then try again.")
		return nil
	}

	// ── Step 3: Select or create a COS instance ──────────────────────────────
	_, _ = fmt.Fprintln(stderr, "Fetching COS instances ...")
	instances, err := h.ListCOSInstances()
	if err != nil {
		return fmt.Errorf("failed to list COS instances: %w", err)
	}

	const createNewInstanceSentinel = "__create_new_instance__"
	instanceOptions := []huh.Option[string]{
		huh.NewOption("Create a new COS instance ...", createNewInstanceSentinel),
	}
	for _, inst := range instances {
		instanceOptions = append(instanceOptions, huh.NewOption(inst.Name, inst.CRN))
	}

	selectedInstanceCRN := instanceOptions[0].Value
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a COS instance").
				Options(instanceOptions...).
				Value(&selectedInstanceCRN),
		),
	).WithInput(tty).WithOutput(tty).Run(); err != nil {
		return err
	}

	if selectedInstanceCRN == createNewInstanceSentinel {
		newInst, err := runCreateInstanceFlow(h, tty)
		if err != nil {
			return err
		}
		selectedInstanceCRN = newInst.CRN
	}

	// ── Step 4: Select or create an HMAC resource key ───────────────────────
	_, _ = fmt.Fprintln(stderr, "Fetching HMAC resource keys ...")
	existingKeys, err := h.ListResourceKeys(selectedInstanceCRN)
	if err != nil {
		return fmt.Errorf("failed to list resource keys: %w", err)
	}

	const createNewSentinel = "__create_new__"
	keyOptions := []huh.Option[string]{
		huh.NewOption("Create a new HMAC key ...", createNewSentinel),
	}
	for _, k := range existingKeys {
		keyOptions = append(keyOptions, huh.NewOption(k.Name, k.GUID))
	}

	selectedKeyGUID := keyOptions[0].Value
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select an HMAC resource key").
				Options(keyOptions...).
				Value(&selectedKeyGUID),
		),
	).WithInput(tty).WithOutput(tty).Run(); err != nil {
		return err
	}

	// ── Step 5: Create new key if requested ──────────────────────────────────
	var selectedCreds cosconfighelper.HMACCredentials
	if selectedKeyGUID == createNewSentinel {
		keyName := "go-cache-prog-hmac"
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("New HMAC key name").
					Value(&keyName),
			),
		).WithInput(tty).WithOutput(tty).Run(); err != nil {
			return err
		}

		_, _ = fmt.Fprintln(stderr, "Creating HMAC resource key ...")
		newKey, err := h.CreateResourceKey(selectedInstanceCRN, keyName)
		if err != nil {
			return fmt.Errorf("failed to create resource key: %w", err)
		}
		selectedCreds = newKey.Credentials
	} else {
		for _, k := range existingKeys {
			if k.GUID == selectedKeyGUID {
				selectedCreds = k.Credentials
				break
			}
		}
	}

	// ── Step 6: Select region ─────────────────────────────────────────────────
	// IBM COS SigV4 HMAC signing requires the credential-scope region in the
	// Authorization header to match the endpoint region. ListBuckets scoped to
	// a region only returns buckets homed in that region, so we ask the user
	// upfront rather than picking a hardcoded region and getting empty results.
	regionOptions := make([]huh.Option[string], len(cosconfighelper.KnownRegions))
	for i, r := range cosconfighelper.KnownRegions {
		regionOptions[i] = huh.NewOption(r, r)
	}
	region := regionOptions[0].Value
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Bucket region").
				Description("IBM COS buckets are regional — select the region your bucket is in (or will be created in)").
				Options(regionOptions...).
				Value(&region),
		),
	).WithInput(tty).WithOutput(tty).Run(); err != nil {
		return err
	}

	// ── Step 7: List and select bucket ───────────────────────────────────────
	_, _ = fmt.Fprintln(stderr, "Fetching buckets ...")
	buckets, err := h.ListBuckets(selectedCreds.AccessKeyID, selectedCreds.SecretAccessKey, region)
	if err != nil {
		return fmt.Errorf("failed to list buckets: %w", err)
	}

	const createNewBucketSentinel = "__create_new_bucket__"
	bucketOptions := []huh.Option[string]{
		huh.NewOption("Create a new bucket ...", createNewBucketSentinel),
	}
	for _, b := range buckets {
		bucketOptions = append(bucketOptions, huh.NewOption(b, b))
	}

	selectedBucket := bucketOptions[0].Value
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a bucket").
				Options(bucketOptions...).
				Value(&selectedBucket),
		),
	).WithInput(tty).WithOutput(tty).Run(); err != nil {
		return err
	}

	// ── Step 8: Create new bucket if requested ───────────────────────────────
	if selectedBucket == createNewBucketSentinel {
		var newBucket string
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("New bucket name").
					Description("Bucket names must be globally unique across all IBM COS instances").
					Value(&newBucket),
			),
		).WithInput(tty).WithOutput(tty).Run(); err != nil {
			return err
		}

		_, _ = fmt.Fprintln(stderr, "Creating bucket ...")
		if err := h.CreateBucket(selectedCreds.AccessKeyID, selectedCreds.SecretAccessKey, newBucket, region); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		selectedBucket = newBucket
	}

	// ── Step 9: Choose endpoint variant ──────────────────────────────────────
	publicEndpoint := fmt.Sprintf("s3.%s.cloud-object-storage.appdomain.cloud", region)
	privateEndpoint := fmt.Sprintf("s3.private.%s.cloud-object-storage.appdomain.cloud", region)
	directEndpoint := fmt.Sprintf("s3.direct.%s.cloud-object-storage.appdomain.cloud", region)

	selectedEndpoint := publicEndpoint
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select endpoint variant").
				Options(
					huh.NewOption(fmt.Sprintf("Public  (%s)", publicEndpoint), publicEndpoint),
					huh.NewOption(fmt.Sprintf("Private (%s)", privateEndpoint), privateEndpoint),
					huh.NewOption(fmt.Sprintf("Direct  (%s)", directEndpoint), directEndpoint),
				).
				Value(&selectedEndpoint),
		),
	).WithInput(tty).WithOutput(tty).Run(); err != nil {
		return err
	}

	// ── Step 10: Local cache directory ───────────────────────────────────────
	cacheDir := "/tmp/go-cache"
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Local cache directory").
				Description("Directory used to cache objects locally on disk").
				Value(&cacheDir),
		),
	).WithInput(tty).WithOutput(tty).Run(); err != nil {
		return err
	}

	// ── Step 11: Serialise and output ────────────────────────────────────────
	cfg := cos.Config{
		CacheDir: cacheDir,
		Cos: cos.Cos{
			Endpoint:        selectedEndpoint,
			Region:          region,
			Bucket:          selectedBucket,
			AccessKeyID:     selectedCreds.AccessKeyID,
			SecretAccessKey: selectedCreds.SecretAccessKey,
		},
	}

	out, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to serialise config: %w", err)
	}

	if cosConfigHelperSettings.outputFile != "" {
		if err := os.WriteFile(cosConfigHelperSettings.outputFile, out, 0600); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		_, _ = fmt.Fprintf(stderr, "Config written to %s\n", cosConfigHelperSettings.outputFile)
		return nil
	}

	fmt.Println(string(out))
	return nil
}

// runCreateInstanceFlow is the interactive sub-flow for creating a new COS
// service instance. It prompts for a name, resource group, and plan, then
// calls CreateCOSInstance and polls until active.
// COS is a global service — no region selection needed.
func runCreateInstanceFlow(h *cosconfighelper.COSConfigHelper, tty *os.File) (cosconfighelper.COSInstance, error) {
	_, _ = fmt.Fprintln(tty, "Fetching resource groups and plans ...")

	groups, err := h.ListResourceGroups()
	if err != nil {
		return cosconfighelper.COSInstance{}, fmt.Errorf("failed to list resource groups: %w", err)
	}
	if len(groups) == 0 {
		return cosconfighelper.COSInstance{}, fmt.Errorf("no resource groups found — check your account permissions")
	}

	catalogEntry, err := h.FetchCOSCatalogEntry()
	if err != nil {
		return cosconfighelper.COSInstance{}, fmt.Errorf("failed to fetch COS catalog entry: %w", err)
	}

	groupOptions := make([]huh.Option[string], len(groups))
	for i, g := range groups {
		groupOptions[i] = huh.NewOption(g.Name, g.ID)
	}

	planOptions := make([]huh.Option[string], 0, len(catalogEntry.Plans))
	for _, p := range catalogEntry.Plans {
		if p.ID == "" {
			continue
		}
		planOptions = append(planOptions, huh.NewOption(p.Name, p.ID))
	}
	if len(planOptions) == 0 {
		return cosconfighelper.COSInstance{}, fmt.Errorf("no provisionable COS plans found")
	}

	instanceName := "go-cache-prog-cos"
	selectedGroupID := groups[0].ID
	selectedPlanID := planOptions[0].Value

	// Only show the resource group select when there is more than one choice.
	formFields := []huh.Field{
		huh.NewInput().
			Title("New COS instance name").
			Value(&instanceName),
	}
	if len(groups) > 1 {
		formFields = append(formFields,
			huh.NewSelect[string]().
				Title("Resource group").
				Options(groupOptions...).
				Value(&selectedGroupID),
		)
	} else {
		_, _ = fmt.Fprintf(tty, "Using resource group: %s\n", groups[0].Name)
	}
	formFields = append(formFields,
		huh.NewSelect[string]().
			Title("Service plan").
			Options(planOptions...).
			Value(&selectedPlanID),
	)

	if err := huh.NewForm(
		huh.NewGroup(formFields...),
	).WithInput(tty).WithOutput(tty).Run(); err != nil {
		return cosconfighelper.COSInstance{}, err
	}

	_, _ = fmt.Fprintln(tty, "Creating COS instance ...")
	return h.CreateCOSInstance(instanceName, selectedGroupID, selectedPlanID, tty)
}
