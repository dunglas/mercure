/** Load test for Mercure.
  *
  *   1. Grab Gatling 3 on https://gatling.io 2. Run
  *      path/to/gatling/bin/gatling.sh --simulations-folder .
  *
  * Available environment variables (all optional):
  *   - HUB_URL: the URL of the hub to test
  *   - JWT: the JWT to use for authenticating the publisher, fallbacks to JWT
  *     if not set and PRIVATE_UPDATES set
  *   - INITIAL_SUBSCRIBERS: the number of concurrent subscribers initially
  *     connected
  *   - SUBSCRIBERS_RATE_FROM: minimum rate (per second) of additional
  *     subscribers to connect
  *   - SUBSCRIBERS_RATE_TO: maximum rate (per second) of additional subscribers
  *     to connect
  *   - PUBLISHERS_RATE_FROM: minimum rate (per second) of publications
  *   - PUBLISHERS_RATE_TO: maximum rate (per second) of publications
  *   - INJECTION_DURATION: duration of the publishers injection
  *   - CONNECTION_DURATION: duration of subscribers' connection
  *   - RANDOM_CONNECTION_DURATION: to randomize the connection duration (will
  *     longs CONNECTION_DURATION at max)
  */

package mercure

import io.gatling.core.Predef._
import io.gatling.http.Predef._
import scala.concurrent.duration._
import scala.util.Properties

class LoadTest extends Simulation {

  /** The hub URL */
  val HubUrl =
    Properties.envOrElse("HUB_URL", "https://localhost/.well-known/mercure")

  /** JWT to use to publish */
  val Jwt = Properties.envOrElse(
    "JWT",
    "eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.KKPIikwUzRuB3DTpVw6ajzwSChwFw5omBMmMcWKiDcM"
  )

  /** JWT to use to subscribe, fallbacks to JWT if not set and PRIVATE_UPDATES
    * set
    */
  val SubscriberJwt = Properties.envOrElse("SUBSCRIBER_JWT", null)

  /** Number of concurrent subscribers initially connected */
  val InitialSubscribers =
    Properties.envOrElse("INITIAL_SUBSCRIBERS", "100").toInt

  /** Additional subscribers rate (per second) */
  val SubscribersRateFrom =
    Properties.envOrElse("SUBSCRIBERS_RATE_FROM", "2").toInt
  val SubscribersRateTo =
    Properties.envOrElse("SUBSCRIBERS_RATE_TO", "10").toInt

  /** Publishers rate (per second) */
  val PublishersRateFrom =
    Properties.envOrElse("PUBLISHERS_RATE_FROM", "2").toInt
  val PublishersRateTo = Properties.envOrElse("PUBLISHERS_RATE_TO", "20").toInt

  /** Duration of injection (in seconds) */
  val InjectionDuration =
    Properties.envOrElse("INJECTION_DURATION", "3600").toInt

  /** How long a subscriber can stay connected at max (in seconds) */
  val ConnectionDuration =
    Properties.envOrElse("CONNECTION_DURATION", "300").toInt

  /** Randomize the connection duration? */
  val RandomConnectionDuration =
    Properties.envOrElse("RANDOM_CONNECTION_DURATION", "true").toBoolean

  /** Send private updates with random topics instead of public ones always with
    * the same topic
    */
  var PrivateUpdates =
    Properties.envOrElse("PRIVATE_UPDATES", "false").toBoolean

  val rnd = new scala.util.Random

  /** Subscriber test as a function to handle conditional Authorization header
    */
  def subscriberTest() = {
    var topic = "https://example.com"
    if (PrivateUpdates) {
      topic = topic + "/{id}"
    }

    var requestBuilder = sse("Subscribe").connect("?topic=" + topic)

    if (SubscriberJwt != null) {
      requestBuilder =
        requestBuilder.header("Authorization", "Bearer " + SubscriberJwt)
    } else if (PrivateUpdates) {
      requestBuilder = requestBuilder.header("Authorization", "Bearer " + Jwt)
    }

    requestBuilder.await(10)(
      sse.checkMessage("Check content").check(regex("""(.*)Hi(.*)"""))
    )
  }

  val httpProtocol = http
    .baseUrl(HubUrl)

  var topic = "https://example.com"
  if (PrivateUpdates) {
    topic = topic + "/" + rnd.nextInt()
  }

  var data = Map("topic" -> topic, "data" -> "Hi")
  if (PrivateUpdates) {
    data = data + ("private" -> "true")
  }

  val scenarioPublish = scenario("Publish")
    .exec(
      http("Publish")
        .post("")
        .header("Authorization", "Bearer " + Jwt)
        .formParamMap(data)
        .check(status.is(200))
    )

  val scenarioSubscribe = scenario("Subscribe")
    .exec(
      subscriberTest()
    )
    .pause(
      if (RandomConnectionDuration) rnd.nextInt(ConnectionDuration)
      else ConnectionDuration
    )
    .exec(sse("Close").close)

  setUp(
    scenarioSubscribe
      .inject(
        atOnceUsers(InitialSubscribers),
        rampUsersPerSec(
          SubscribersRateFrom
        ) to SubscribersRateTo during (InjectionDuration seconds) randomized
      )
      .protocols(httpProtocol),
    scenarioPublish
      .inject(
        rampUsersPerSec(
          PublishersRateFrom
        ) to PublishersRateTo during (InjectionDuration + ConnectionDuration seconds) randomized
      )
      .protocols(httpProtocol)
  )
}
