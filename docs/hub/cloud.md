# Mercure.rocks Cloud Service

Mercure.rocks provides a managed cloud service enabling instant deployment of hosted hubs, without the need for operational or DevOps skills.
Our SRE team constantly monitors hubs. Updates are automatically applied.

Unlike the free and open-source hubs, Cloud hubs use [clustering](cluster.md) and are deployed in a high-availability infrastructure.

The Cloud service is built on top of the free and open-source hub and helps fund its development.

## Subscribing

Purchase your managed Mercure.rocks hub [directly online](https://mercure.rocks/pricing)!

After purchase, your hub will be instantly provisionned and available under a `mercure.rocks` subdomain. A TLS certificate is also automatically created.

You'll have access to an administration interface allowing you to:

* configure your hub (the same [configuration settings](config.md) as for the free and open-source hub are available)
* access to the logs of your hub
* [set a custom domain name](#custom-domain)
* [switch to another plan](#switching-between-plans)

## Custom Domain

Your managed hub can be associated with a custom domain. A TLS certificate is also automatically created for your custom domain.

If you use the cookie-based authentication mechanism, it is necessary to associate your hub with a subdomain of your website's main domain [to avoid CORS problems](troubleshooting.md#cors-issues).

To associate your personalized domain with your hub:

1. From the administration interface provided by your domain registrar, add a `CNAME` DNS entry pointing to the domain name ending in `.mercure.rocks.` displayed in the Mercure.rocks administration interface.
2. Define your custom domain name in the Mercure.rocks administration interface.

The DNS entry and TLS certificate may take up to 24 hours to become available.
In general, it only takes a few minutes.

## Switching Between Plans

If you need more or fewer resources, you can switch [from one plan to another](https://mercure.rocks/pricing) at any time from the administration interface.

The switch is made without service interruption.

## Rate Limiting

If you reach [your current plan limits](https://mercure.rocks/pricing), the hub will return HTTP status code 429 (`Too Many Requests`).

Publication requests will be rejected and subscription requests will fail.

Be sure to catch and logs these errors in your code.

If you need more requests, upgrade to a higher plan or [the On Premise version](#on-premise).

## On Premise

The [high availability hub](cluster.md) we use for the cloud service can also be hosted on your own infrastructure. When you use the [on-premise version](cluster.md#high-availability-on-premise-version), there are no limits other than the load that can be handled by your servers.

[Contact us for more information on the On Premise version](mailto:contact@mercure.rocks?subject=I%27m%20interested%20in%20Mercure%20on%20premise).

## Support

For support requests related to the Cloud version of Mercure.rocks, send a mail to [contact@mercure.rocks](mailto:contact@mercure.rocks?subject=Cloud%20support%20request).
Please include the ID of your hub in the message.
