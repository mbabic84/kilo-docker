package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// createOSUser creates an OS user with the given name and UID/GID.
func createOSUser(username, uidStr, gidStr string) error {
	_ = exec.Command("deluser", username).Run()
	cmd := exec.Command("addgroup", "--gid", gidStr, username)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("addgroup: %v: %s", err, out)
	}
	cmd = exec.Command("adduser", "--uid", uidStr, "--ingroup", username, "--disabled-password", "--shell", "/bin/sh", username)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adduser: %v: %s", err, out)
	}
	return nil
}

// joinHostGroups adds the container user to supplementary groups matching
// the host user's group IDs (passed via PGIDS env var). This enables access
// to workspace files with group-level permissions after privilege drop.
func joinHostGroups(username string) {
	pgidsStr := os.Getenv("PGIDS")
	if pgidsStr == "" {
		utils.Log("[userinit] joinHostGroups: no PGIDS env var\n")
		return
	}
	utils.Log("[userinit] joinHostGroups: host supplementary groups: %s\n", pgidsStr)

	var groupNames []string
	for _, gidStr := range strings.Split(pgidsStr, ",") {
		gidStr = strings.TrimSpace(gidStr)
		if gidStr == "" {
			continue
		}
		out, err := exec.Command("getent", "group", gidStr).Output()
		if err == nil {
			parts := strings.SplitN(strings.TrimSpace(string(out)), ":", 2)
			if len(parts) > 0 && parts[0] != "" {
				groupNames = append(groupNames, parts[0])
			}
		} else {
			groupName := "kilo-host-gid-" + gidStr
			if err := exec.Command("addgroup", "--gid", gidStr, groupName).Run(); err != nil {
				utils.LogWarn("[userinit] joinHostGroups: failed to create group %s (GID %s): %v\n", groupName, gidStr, err)
				continue
			}
			utils.Log("[userinit] joinHostGroups: created group %s with GID %s\n", groupName, gidStr)
			groupNames = append(groupNames, groupName)
		}
	}

	if len(groupNames) == 0 {
		utils.Log("[userinit] joinHostGroups: no groups to join\n")
		return
	}

	joined := strings.Join(groupNames, ",")
	utils.Log("[userinit] joinHostGroups: adding %s to groups: %s\n", username, joined)
	if err := exec.Command("usermod", "--append", "--groups", joined, username).Run(); err != nil {
		utils.LogWarn("[userinit] joinHostGroups: failed to add %s to groups: %v\n", username, err)
	} else {
		utils.Log("[userinit] joinHostGroups: added %s to groups: %s\n", username, joined)
	}
}

// getUserGroups returns a list of supplementary group IDs for the given user.
// Uses getent to read the group database.
func getUserGroups(username string) []int {
	var groups []int

	// Read /etc/group to find all groups the user belongs to
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		utils.LogWarn("[userinit] Failed to read /etc/group: %v\n", err)
		return groups
	}

	for _, line := range strings.Split(string(data), "\n") {
		// Format: groupname:password:GID:user1,user2,...
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}

		gid, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}

		// Check if user is in this group
		members := strings.Split(parts[3], ",")
		for _, member := range members {
			if strings.TrimSpace(member) == username {
				groups = append(groups, gid)
				break
			}
		}
	}

	return groups
}
