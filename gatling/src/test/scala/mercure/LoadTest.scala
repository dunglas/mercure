/** Load test for Mercure.
  *
  * Run the load test: ./mvnw gatling:test
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
    "eyJhbGciOiJIUzI1NiIsInR5cCI6ImF0K2p3dCJ9.eyJhdWQiOiJodHRwczovL2xvY2FsaG9zdC8ud2VsbC1rbm93bi9tZXJjdXJlIiwiYXV0aG9yaXphdGlvbl9kZXRhaWxzIjpbeyJhY3Rpb25zIjpbInB1Ymxpc2giXSwidG9waWNzIjpbeyJtYXRjaCI6IioifV0sInR5cGUiOiJodHRwczovL21lcmN1cmUucm9ja3MvYXV0aG9yaXphdGlvbi1kZXRhaWwifSx7ImFjdGlvbnMiOlsic3Vic2NyaWJlIl0sInRvcGljcyI6W3sibWF0Y2giOiIqIn1dLCJ0eXBlIjoiaHR0cHM6Ly9tZXJjdXJlLnJvY2tzL2F1dGhvcml6YXRpb24tZGV0YWlsIn1dLCJleHAiOjQxMDI0NDQ4MDAsImlzcyI6Imh0dHBzOi8vbG9jYWxob3N0In0.VO0-PRjJ2MGOrMk2HxlrBv217pB7hyLxLIQUGgSfyXs"
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

  /** Override the subscribe matcher query parameter, e.g.
    * "match=https://example.com" (exact) or
    * "match_urlpattern=https://example.com/:id" (URL Pattern). Lets a run
    * target a specific matcher type without editing this file; defaults to a
    * value derived from PRIVATE_UPDATES.
    */
  val SubscribeParam = Properties.envOrElse("SUBSCRIBE_PARAM", null)

  /** Override the published topic. Must stay matchable by the subscriber's
    * matcher, otherwise the delivery check times out. Defaults to a value
    * derived from PRIVATE_UPDATES.
    */
  val PublishTopic = Properties.envOrElse("PUBLISH_TOPIC", null)

  val rnd = new scala.util.Random

  /** Subscriber test as a function to handle conditional Authorization header
    */
  def subscriberTest() = {
    // Public updates share one exact topic; private updates use random topics
    // under a URL Pattern. SUBSCRIBE_PARAM overrides both.
    var param = "match=https://example.com"
    if (PrivateUpdates) {
      param = "match_urlpattern=https://example.com/:id"
    }
    if (SubscribeParam != null) {
      param = SubscribeParam
    }

    var requestBuilder = sse("Subscribe").get("?" + param)

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
  if (PublishTopic != null) {
    topic = PublishTopic
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
