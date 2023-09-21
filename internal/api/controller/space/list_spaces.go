// Copyright 2023 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package space

import (
	"context"
	"fmt"

	apiauth "github.com/harness/gitness/internal/api/auth"
	"github.com/harness/gitness/internal/auth"
	"github.com/harness/gitness/store/database/dbtx"
	"github.com/harness/gitness/types"
	"github.com/harness/gitness/types/enum"
)

// ListSpaces lists the child spaces of a space.
func (c *Controller) ListSpaces(ctx context.Context,
	session *auth.Session,
	spaceRef string,
	filter *types.SpaceFilter,
) ([]*types.Space, int64, error) {
	space, err := c.spaceStore.FindByRef(ctx, spaceRef)
	if err != nil {
		return nil, 0, err
	}

	if err = apiauth.CheckSpace(ctx, c.authorizer, session, space, enum.PermissionSpaceView, true); err != nil {
		return nil, 0, err
	}
	return c.ListSpacesNoAuth(ctx, space.ID, filter)
}

// ListSpacesNoAuth lists spaces WITHOUT checking PermissionSpaceView.
func (c *Controller) ListSpacesNoAuth(
	ctx context.Context,
	spaceID int64,
	filter *types.SpaceFilter,
) ([]*types.Space, int64, error) {
	var spaces []*types.Space
	var count int64

	err := dbtx.New(c.db).WithTx(ctx, func(ctx context.Context) (err error) {
		count, err = c.spaceStore.Count(ctx, spaceID, filter)
		if err != nil {
			return fmt.Errorf("failed to count child spaces: %w", err)
		}

		spaces, err = c.spaceStore.List(ctx, spaceID, filter)
		if err != nil {
			return fmt.Errorf("failed to list child spaces: %w", err)
		}

		return nil
	}, dbtx.TxDefaultReadOnly)
	if err != nil {
		return nil, 0, err
	}

	return spaces, count, nil
}
