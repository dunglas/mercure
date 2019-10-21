/** Load test for Mercure.
 *
 * 1. Grab Gatling 3 on https://gatling.io
 * 2. Run path/to/gatling/bin/gatling.sh --simulations-folder .
 *
 * Available environment variables (all optional):
 *   - HUB_URL: the URL of the hub to test
 *   - JWT: the JWT to use for authenticating the publisher
 *   - SUBSCRIBERS: the number of concurrent subscribers
 *   - PUBLISHERS: the number of concurrent publishers
*/

package mercure

import io.gatling.core.Predef._
import io.gatling.http.Predef._
import scala.concurrent.duration._
import scala.util.Properties

class LoadTest extends Simulation {
  /** The hub URL */
  val HubUrl = Properties.envOrElse("HUB_URL", "http://localhost:3001/.well-known/mercure" )
  /** JWT to use to publish */
  val Jwt = Properties.envOrElse("JWT", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJmb28iLCJiYXIiXSwicHVibGlzaCI6WyJmb28iXX19.afLx2f2ut3YgNVFStCx95Zm_UND1mZJ69OenXaDuZL8")
  /** Number of concurrent subscribers to connect */
  val ConcurrentSubscribers = Properties.envOrElse("SUBSCRIBERS", "10000").toInt
  /** Number of concurent publishers */
  val ConcurrentPublishers = Properties.envOrElse("PUBLISHERS", "2").toInt

  val httpProtocol = http
    .baseUrl(HubUrl)

  val scenarioPublish = scenario("Publish")
    .pause(2) // Wait for subscribers
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
    .pause(15)
    .exec(sse("Close").close())

  setUp(
    scenarioSubscribe.inject(atOnceUsers(ConcurrentSubscribers)).protocols(httpProtocol),
    scenarioPublish.inject(atOnceUsers(ConcurrentPublishers)).protocols(httpProtocol)
  )
}
