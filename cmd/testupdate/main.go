package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/cloudwithax/swarmy/internal/update"
)

func main() {
	// Test platform detection
	platform := update.CurrentPlatform()
	fmt.Printf("Platform: OS=%s, Arch=%s\n", platform.OS, platform.Arch)
	fmt.Printf("Asset name: %s\n", platform.AssetName("v0.0.0-nightly.20260318215436"))

	// Test checking for updates
	ctx := context.Background()
	info, err := update.CheckNightlyInfo(ctx, "v0.0.0-nightly.20260318215436")
	if err != nil {
		fmt.Printf("CheckNightlyInfo error: %v\n", err)
	} else {
		fmt.Printf("Update info: Current=%s, Latest=%s, Available=%v\n",
			info.Current, info.Latest, info.Available())
	}

	// Show what asset we're looking for
	fmt.Printf("\nGOOS=%s, GOARCH=%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Looking for asset: %s\n", update.GetAssetName())
}
