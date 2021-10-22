package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"t0ast.cc/tbml/internal"
	uerror "t0ast.cc/tbml/util/error"
)

type LsCmd struct{}

func (cmd *LsCmd) Run(common CommandContext) error {
	instances, err := internal.GetProfileInstances(common.Config)
	if err != nil {
		return uerror.WithStackTrace(err)
	}

	instancesPerProfile := make(map[string][]internal.ProfileInstance)
	for _, instance := range instances {
		is, ok := instancesPerProfile[instance.ProfileLabel]
		if !ok {
			instancesPerProfile[instance.ProfileLabel] = []internal.ProfileInstance{instance}
		}
		instancesPerProfile[instance.ProfileLabel] = append(is, instance)
	}

	sort.Slice(common.Config.Profiles, func(i, j int) bool {
		a, b := common.Config.Profiles[i], common.Config.Profiles[j]
		return a.Label < b.Label
	})

	sb := strings.Builder{}
	for _, profile := range common.Config.Profiles {
		sb.WriteString(profile.Label)

		sb.WriteString(" (user.js? ")
		if profile.UserJSFile == nil {
			sb.WriteString("NO")
		} else {
			sb.WriteString("YES")
		}
		sb.WriteString("; userChrome.css? ")
		if profile.UserChromeFile == nil {
			sb.WriteString("NO")
		} else {
			sb.WriteString("YES")
		}

		if len(profile.ExtensionFiles) > 0 {
			sb.WriteString("; ")
			for i, extensionFile := range profile.ExtensionFiles {
				sb.WriteString(filepath.Base(extensionFile))
				if i < len(profile.ExtensionFiles)-1 {
					sb.WriteString(", ")
				}
			}
		}

		sb.WriteString(")")

		writeColumn := func(str string, width int) {
			sb.WriteString(str)
			spacing := width - len(str)
			if spacing < 2 {
				spacing = 2
			}
			sb.WriteString(strings.Repeat(" ", spacing))
		}

		instances, ok := instancesPerProfile[profile.Label]
		if ok {
			sb.WriteString("\n  │   ")
			writeColumn("Instance", 15)
			writeColumn("Cur. Topic", 15)
			writeColumn("Cur. PID", 15)
			writeColumn("Created", 20)
			writeColumn("Last used", 20)

			for i, instance := range instances {
				sb.WriteString("\n  ")
				if i < len(instances)-1 {
					sb.WriteString("├")
				} else {
					sb.WriteString("└")
				}
				sb.WriteString("── ")
				writeColumn(instance.InstanceLabel, 15)
				if instance.UsageLabel == nil {
					writeColumn("<none>", 15)
				} else {
					writeColumn(*instance.UsageLabel, 15)
				}
				if instance.UsagePID == nil {
					writeColumn("<none>", 15)
				} else {
					writeColumn(strconv.Itoa(*instance.UsagePID), 15)
				}
				writeColumn(instance.Created.Format(time.Stamp), 20)
				writeColumn(instance.LastUsed.Format(time.Stamp), 20)
			}
		}
	}

	fmt.Println(sb.String())
	return nil
}
