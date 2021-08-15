// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/fatih/color"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"gopkg.in/yaml.v2"
)

// ChangeConfig use the cluster's config given by the user.
func (m *Manager) ChangeConfig(name string, config string, skipConfirm bool) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	// Get old metadata
	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return err
	}

	origTopo := metadata.GetTopology()

	// Get new metadata from config file
	newData, err := os.ReadFile(config)
	if err != nil {
		return perrs.AddStack(err)
	}

	newTopo := m.specManager.NewMetadata().GetTopology()
	err = yaml.UnmarshalStrict(newData, newTopo)
	if err != nil {
		fmt.Print(color.RedString("New topology could not be saved: "))
		log.Infof("Failed to parse topology file: %v", err)
		log.Infof("Nothing changed.")
		return perrs.AddStack(err)
	}

	// report error if immutable field has been changed
	if err := utils.ValidateSpecDiff(origTopo, newTopo); err != nil {
		fmt.Print(color.RedString("New topology could not be saved: "))
		log.Errorf("%s", err)
		log.Infof("Nothing changed.")
		return perrs.AddStack(err)
	}

	origData, err := yaml.Marshal(origTopo)
	if err != nil {
		return perrs.AddStack(err)
	}

	if bytes.Equal(origData, newData) {
		log.Infof("The file has nothing changed")
		return nil
	}

	utils.ShowDiff(string(origData), string(newData), os.Stdout)

	if !skipConfirm {
		if err := tui.PromptForConfirmOrAbortError(
			color.HiYellowString("Please check change highlight above, do you want to apply the change? [y/N]:"),
		); err != nil {
			return err
		}
	}

	log.Infof("Applying changes...")
	metadata.SetTopology(newTopo)
	err = m.specManager.SaveMeta(name, metadata)
	if err != nil {
		return perrs.Annotate(err, "failed to save meta")
	}

	log.Infof("Applied successfully, please use `%s reload %s [-N <nodes>] [-R <roles>]` to reload config.", tui.OsArgs0(), name)
	return nil
}
