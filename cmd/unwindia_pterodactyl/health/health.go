package health

import (
	"github.com/GSH-LAN/Unwindia_pterodactyl/cmd/unwindia_pterodactyl/database"
	"github.com/alexliesenfeld/health"
	"github.com/apache/pulsar-client-go/pulsar"
	httpCheck "github.com/hellofresh/health-go/v4/checks/http"
	"net/http"
	"time"
)

type Checker struct {
	checker health.Checker
}

func NewChecker(db *database.DatabaseClient, pteroURL string, pulsarClient pulsar.Client) *Checker {
	checker := health.NewChecker(

		// Set the time-to-live for our cache to 1 second (default).
		health.WithCacheDuration(1*time.Second),

		// Configure a global timeout that will be applied to all checks.
		health.WithTimeout(10*time.Second),

		//// A check configuration to see if our database connection is up.
		//// The check function will be executed for each HTTP request.
		//health.WithCheck(health.Check{
		//	Name:    "database",      // A unique check name.
		//	Timeout: 2 * time.Second, // A check specific timeout.
		//	Check:   db.PingContext,
		//}),

		// The following check will be executed periodically every 15 seconds
		// started with an initial delay of 3 seconds. The check function will NOT
		// be executed for each HTTP request.
		health.WithPeriodicCheck(15*time.Second, 3*time.Second, health.Check{
			Name: "pterodactyl-panel",
			// The check function checks the health of a component. If an error is
			// returned, the component is considered unavailable (or "down").
			// The context contains a deadline according to the configured timeouts.
			Check: httpCheck.New(httpCheck.Config{
				URL: pteroURL,
			}),
		}),

		// Set a status listener that will be invoked when the health status changes.
		// More powerful hooks are also available (see docs).
		//health.WithStatusListener(func(ctx context.Context, state health.CheckerState) {
		//	log.Println(fmt.Sprintf("health status changed to %s", state.Status))
		//}),
	)

	c := Checker{
		checker,
	}

	return &c
}

func (c *Checker) GetHealthHandler() http.Handler {
	return health.NewHandler(c.checker)
}
