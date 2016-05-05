// Copyright 2012 Jimmy Zelinskie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2016 Samir Bhatt. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package geddit

import (
	"testing"
)

// TODO: Write better test functions

func TestMain(t *testing.T) {
	a, err := NewAppOnlyOAuthSession(
		"client_id",
		"client_secret",
		"Testing OAuth Bot by u/imheresamir v0.1 see source https://github.com/imheresamir/geddit",
		false,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Ready to make API calls!
	_, err = a.SubredditSubmissions("hiphopheads", "hot", ListingOptions{})

	if err != nil {
		t.Fatal(err)
	}

}
