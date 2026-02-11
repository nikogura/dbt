package dbt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// PurgeOptions holds the options for a purge operation.
type PurgeOptions struct {
	ToolName  string
	All       bool
	OlderThan time.Duration
	Keep      int
	DryRun    bool
	Yes       bool
}

// FetchVersionMetadata fetches version metadata from the reposerver API.
func (dbt *DBT) FetchVersionMetadata(toolName string) (versions []VersionInfo, err error) {
	repoBase := strings.TrimSuffix(dbt.Config.Tools.Repo, "/")
	// Strip the trailing path component (e.g., /dbt-tools) to get the server base
	// The API is at /-/api/tools/<name>/versions
	serverBase := repoBase
	if idx := strings.LastIndex(serverBase, "/"); idx > 0 {
		serverBase = serverBase[:idx]
	}

	uri := fmt.Sprintf("%s/-/api/tools/%s/versions", serverBase, toolName)

	client := &http.Client{
		Timeout: defaultRequestTimeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if reqErr != nil {
		err = errors.Wrapf(reqErr, "failed to create request for %s", uri)
		return versions, err
	}

	err = dbt.AuthHeaders(req)
	if err != nil {
		err = errors.Wrapf(err, "failed adding auth headers")
		return versions, err
	}

	resp, doErr := client.Do(req)
	if doErr != nil {
		err = errors.Wrapf(doErr, "failed to fetch version metadata from %s", uri)
		return versions, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		err = fmt.Errorf("tool %s not found", toolName)
		return versions, err
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("API returned status %d for %s", resp.StatusCode, uri)
		return versions, err
	}

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		err = errors.Wrap(readErr, "failed to read API response")
		return versions, err
	}

	err = json.Unmarshal(bodyBytes, &versions)
	if err != nil {
		err = errors.Wrap(err, "failed to parse version metadata")
		return versions, err
	}

	return versions, err
}

// DeleteToolVersion deletes a specific version of a tool from the repository.
func (dbt *DBT) DeleteToolVersion(toolName string, version string) (err error) {
	targetURL := fmt.Sprintf("%s/%s/%s", dbt.Config.Tools.Repo, toolName, version)

	_, deleteErr := dbt.DeletePath(targetURL)
	if deleteErr != nil {
		err = errors.Wrapf(deleteErr, "failed to delete version %s of %s", version, toolName)
		return err
	}

	return err
}

// DeleteTool deletes an entire tool from the repository.
func (dbt *DBT) DeleteTool(toolName string) (err error) {
	targetURL := fmt.Sprintf("%s/%s", dbt.Config.Tools.Repo, toolName)

	_, deleteErr := dbt.DeletePath(targetURL)
	if deleteErr != nil {
		err = errors.Wrapf(deleteErr, "failed to delete tool %s", toolName)
		return err
	}

	return err
}

// confirmAction prompts the user for confirmation and returns true if confirmed.
func confirmAction(message string) (confirmed bool) {
	fmt.Print(message)

	var response string

	_, scanErr := fmt.Scanln(&response)
	if scanErr != nil {
		confirmed = false
		return confirmed
	}

	confirmed = response == "yes"

	return confirmed
}

// purgeAllVersions handles the --all flag for PurgeTool.
func (dbt *DBT) purgeAllVersions(opts PurgeOptions) (err error) {
	if !opts.Yes {
		msg := fmt.Sprintf("WARNING: This will delete the ENTIRE tool %q and all its versions.\nType 'yes' to confirm: ", opts.ToolName)
		if !confirmAction(msg) {
			fmt.Println("Aborted.")
			return err
		}
	}

	if opts.DryRun {
		fmt.Printf("[dry-run] Would delete entire tool %q\n", opts.ToolName)
		return err
	}

	err = dbt.DeleteTool(opts.ToolName)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted tool %q\n", opts.ToolName)

	return err
}

// PurgeTool orchestrates the purge operation based on the provided options.
func (dbt *DBT) PurgeTool(opts PurgeOptions) (err error) {
	if opts.All {
		err = dbt.purgeAllVersions(opts)
		return err
	}

	// Fetch version metadata
	versions, fetchErr := dbt.FetchVersionMetadata(opts.ToolName)
	if fetchErr != nil {
		err = fetchErr
		return err
	}

	if len(versions) == 0 {
		fmt.Printf("No versions found for %s\n", opts.ToolName)
		return err
	}

	toDelete := selectVersionsForDeletion(versions, opts)

	if len(toDelete) == 0 {
		fmt.Printf("No versions match the deletion criteria for %s\n", opts.ToolName)
		return err
	}

	printDeletionSummary(toDelete, versions, opts.ToolName)

	if !opts.Yes {
		if !confirmAction("Type 'yes' to confirm: ") {
			fmt.Println("Aborted.")
			return err
		}
	}

	for _, v := range toDelete {
		if opts.DryRun {
			fmt.Printf("[dry-run] Would delete %s/%s\n", opts.ToolName, v.Version)
			continue
		}

		deleteErr := dbt.DeleteToolVersion(opts.ToolName, v.Version)
		if deleteErr != nil {
			err = errors.Wrapf(deleteErr, "failed to delete version %s", v.Version)
			return err
		}

		fmt.Printf("Deleted %s/%s\n", opts.ToolName, v.Version)
	}

	return err
}

// printDeletionSummary prints the list of versions that will be deleted.
func printDeletionSummary(toDelete []VersionInfo, allVersions []VersionInfo, toolName string) {
	if len(toDelete) == len(allVersions) {
		fmt.Printf("WARNING: This will remove ALL %d versions of %q (effectively a full purge).\n", len(toDelete), toolName)
	} else {
		fmt.Printf("Will delete %d of %d versions of %q:\n", len(toDelete), len(allVersions), toolName)
	}

	for _, v := range toDelete {
		fmt.Printf("  %s (modified: %s)\n", v.Version, v.ModifiedAt.Format(time.RFC3339))
	}
}

// selectVersionsForDeletion selects which versions should be deleted based on the options.
// Versions are sorted newest-first by semver, then filtered by OlderThan cutoff,
// and capped to preserve the Keep count.
func selectVersionsForDeletion(versions []VersionInfo, opts PurgeOptions) (toDelete []VersionInfo) {
	toDelete = make([]VersionInfo, 0)

	// Sort newest first by semver
	sorted := make([]VersionInfo, len(versions))
	copy(sorted, versions)

	sort.Slice(sorted, func(i, j int) (result bool) {
		result = VersionAIsNewerThanB(sorted[i].Version, sorted[j].Version)
		return result
	})

	// Build a set of versions protected by the Keep constraint.
	// The first opts.Keep versions in sorted order (newest-first) are protected.
	protected := make(map[string]bool)
	if opts.Keep > 0 {
		for i := range sorted {
			if i >= opts.Keep {
				break
			}
			protected[sorted[i].Version] = true
		}
	}

	// Filter by age cutoff and exclude protected versions
	now := time.Now()

	for _, v := range sorted {
		// Skip protected versions
		if protected[v.Version] {
			continue
		}

		if opts.OlderThan > 0 {
			cutoff := now.Add(-opts.OlderThan)
			if v.ModifiedAt.Before(cutoff) {
				toDelete = append(toDelete, v)
			}
		} else {
			toDelete = append(toDelete, v)
		}
	}

	return toDelete
}

// ParseDuration parses a duration string supporting days (d) and weeks (w)
// in addition to the standard Go duration suffixes.
func ParseDuration(s string) (d time.Duration, err error) {
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		days, parseErr := strconv.Atoi(numStr)
		if parseErr != nil {
			err = errors.Wrapf(parseErr, "invalid duration: %s", s)
			return d, err
		}
		d = time.Duration(days) * 24 * time.Hour
		return d, err
	}

	if strings.HasSuffix(s, "w") {
		numStr := strings.TrimSuffix(s, "w")
		weeks, parseErr := strconv.Atoi(numStr)
		if parseErr != nil {
			err = errors.Wrapf(parseErr, "invalid duration: %s", s)
			return d, err
		}
		d = time.Duration(weeks) * 7 * 24 * time.Hour
		return d, err
	}

	var parseErr error
	d, parseErr = time.ParseDuration(s)
	if parseErr != nil {
		err = errors.Wrapf(parseErr, "invalid duration: %s", s)
		return d, err
	}

	return d, err
}
