//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package dockerlocal

import (
	"strings"

	"github.com/wercker/sentcli/core"
	"github.com/wercker/sentcli/util"
)

func NewStep(config *StepConfig, options *PipelineOptions) (Step, error) {
	// NOTE(termie) Special case steps are special
	if s.ID == "internal/docker-push" {
		return NewDockerPushStep(config, options)
	}
	if s.ID == "internal/docker-scratch-push" {
		return NewDockerScratchPushStep(config, options)
	}
	if s.ID == "internal/store-container" {
		return NewStoreContainerStep(config, options)
	}
	if strings.HasPrefix(s.ID, "internal/") {
		if !options.EnableDevSteps {
			util.RootLogger().Warnln("Ignoring dev step:", config.ID)
			return nil, nil
		}
	}
	if options.EnableDevSteps {
		if s.ID == "internal/watch" {
			return NewWatchStep(config, options)
		}
		if s.ID == "internal/shell" {
			return NewShellStep(config, options)
		}
	}
	return core.NewStep(config, options)
}
