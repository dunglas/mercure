/** Load test for Mercure.
 *
 * 1. Grab Gatling 3 on https://gatling.io
 * 2. Run path/to/gatling/bin/gatling.sh --simulations-folder .
 *
 * Available environment variables (all optional):
 *   - HUB_URL: the URL of the hub to test
 *   - JWT: the JWT to use for authenticating the publisher
 *   - INITIAL_SUBSCRIBERS: the number of concurrent subscribers initially connected
 *   - SUBSCRIBERS_RATE_FROM: minimum rate (per second) of additional subscribers to connect
 *   - SUBSCRIBERS_RATE_TO: maximum rate (per second) of additional subscribers to connect
 *   - PUBLISHERS_RATE_FROM: minimum rate (per second) of publications
 *   - PUBLISHERS_RATE_TO: maximum rate (per second) of publications
 *   - INJECTION_DURATION: duration of the publishers injection
 *   - CONNECTION_DURATION: duration of subscribers' connection
 *   - RANDOM_CONNECTION_DURATION: to randomize the connection duration (will longs CONNECTION_DURATION at max)
 */

package mercure

import io.gatling.core.Predef._
import io.gatling.http.Predef._
import scala.concurrent.duration._
import scala.util.Properties

class LoadTest extends Simulation {
  /** The hub URL */
  val HubUrl = Properties.envOrElse("HUB_URL", "http://localhost:3001/.well-known/mercure")
  /** JWT to use to publish */
  val Jwt = Properties.envOrElse("JWT", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwiaHR0cDovL2xvY2FsaG9zdDozMDAwL2RlbW8vYm9va3Mve2lkfS5qc29ubGQiXSwicGF5bG9hZCI6eyJ1c2VyIjoiaHR0cHM6Ly9leGFtcGxlLmNvbS91c2Vycy9kdW5nbGFzIiwicmVtb3RlX2FkZHIiOiIxMjcuMC4wLjEifX19.bRUavgS2H9GyCHq7eoPUL_rZm2L7fGujtyyzUhiOsnw")
  /** Number of concurrent subscribers initially connected */
  val InitialSubscribers = Properties.envOrElse("INITIAL_SUBSCRIBERS", "100").toInt
  /** Additional subscribers rate (per second) */
  val SubscribersRateFrom = Properties.envOrElse("SUBSCRIBERS_RATE_FROM", "2").toInt
  val SubscribersRateTo = Properties.envOrElse("SUBSCRIBERS_RATE_TO", "10").toInt
  /** Publishers rate (per second) */
  val PublishersRateFrom = Properties.envOrElse("PUBLISHERS_RATE_FROM", "2").toInt
  val PublishersRateTo = Properties.envOrElse("PUBLISHERS_RATE_TO", "20").toInt
  /** Duration of injection (in seconds) */
  val InjectionDuration = Properties.envOrElse("INJECTION_DURATION", "3600").toInt
  /** How long a subscriber can stay connected at max (in seconds) */
  val ConnectionDuration = Properties.envOrElse("CONNECTION_DURATION", "300").toInt
  /** Randomize the connection duration? */
  val RandomConnectionDuration = Properties.envOrElse("RANDOM_CONNECTION_DURATION", "true").toBoolean

  val rnd = new scala.util.Random

  val httpProtocol = http
    .baseUrl(HubUrl)

  val scenarioPublish = scenario("Publish")
    .exec(
      http("Publish")
        .post("")
        .header("Authorization", "Bearer "+Jwt)
        .formParamMap(Map("topic" -> "http://example.com", "data" -> "Hi"))
        .check(status.is(200))
    )

  val scenarioSubscribe = scenario("Subscribe")
    .exec(
      sse("Subscribe").connect("?topic=http://example.com")
        .await(10)(
          sse.checkMessage("Check content").check(regex("""(.*)Hi(.*)"""))
        )
    )
    .pause(if (RandomConnectionDuration) rnd.nextInt(ConnectionDuration) else ConnectionDuration)
    .exec(sse("Close").close())

  setUp(
    scenarioSubscribe.inject(
      atOnceUsers(InitialSubscribers),
      rampUsersPerSec(SubscribersRateFrom) to SubscribersRateTo during (InjectionDuration seconds) randomized
    ).protocols(httpProtocol),
    scenarioPublish.inject(
      rampUsersPerSec(PublishersRateFrom) to PublishersRateTo during (InjectionDuration + ConnectionDuration seconds) randomized
    ).protocols(httpProtocol)
  )
}
