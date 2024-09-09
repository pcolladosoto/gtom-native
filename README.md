# Gtom Grafana Datasource
The Gtom Datasource for Grafana runs queries against a MongoDB database and parses tim series collections into Grafana dataframes.
This allows the plugin to leverage Grafana's features such as alerting out of the box.

## Wait! Doesn't this plugin already exist?
Well... yes! There's already an [enterprise plugin][] available that (I guess) does exactly this and much more in a much better
and efficient manner. However, the subscription needed to leverage this plugin is quite steep and the research institution I
work for can't really afford it.

In an effort to save some money whilst still being capable of leveraging our central MongoDB instance (which also contains everything
from inventories to caches) I decided to write a bare-bones datasource capable of querying a MongoDB database to extract information
from [time series collections][].

The downside to this plugin replicating a subset of its enterprise counterpart is that, according to Grafana's [plugin policy][] it
cannot be signed. This translates into a bit more convoluted installation procedure than simply running `grafana-cli`. Read on to
find out how to install the plugin...

## Credit where credit is due
This plugin heavily relies on the codebase backing the [simpod-json-datasource][] plugin. Actually, the frontend GUI is practically
the same with some minor modifications. I would like to warmly thank `@simPod` for all his work and for making it publicly accessible.

Also, the logo has been acquired from [123RF Free Images][]. The logo was made by [captainvector][]. We just made some minor tweaks to
the original in terms of colour.

Finally, even though I had no idea it existed when writing the initial plugin version (if I had known I wouldn't probably have written
this datasource), we did come across the [`grafana-mongodb-community-plugin`][] datasource plugin which apparently implements most of
what we have done. Be sure to check it out as, for the moment, I'm pretty sure its more stable and robust than `gtom`.

## Installation
The installation basically translates into `unzip(1)`ping the built datasource into Grafana's plugin directory. As the plugin **is not**
signed, you'll also need to make sure Grafana 'knows' it's safe to load. Assuming the pluing release is `gtom.zip` the following should
do the trick:

    # unzip(1) the plugin into the plugins directory which, by default, is /var/lib/grafana/plugins
    $ unzip gtom.zip -d /var/lib/grafana/plugins

    # Do some sed(1) magic (or manuallly edit the file) so that /etc/grafana/grafana.ini contains
    [plugins]
    # Enter a comma-separated list of plugin identifiers to identify plugins to load even if they are unsigned. Plugins with modified signatures are never loaded.
    allow_loading_unsigned_plugins = "gtom"

Please bear in mind your installation's plugin directory can be found by checking the value of the `plugins` setting under the `[paths]`
section on `/etc/grafana/grafana.ini`. For a stock installation on AlmaLinux 9:

    $ grep /var/lib/grafana /etc/grafana/grafana.ini
    #################################### Paths ####################################
    [paths]
    # Path to where grafana can store temp files, sessions, and the sqlite3 db (if that is used)
    ;data = /var/lib/grafana

    # Temporary files in `data` directory older than given duration will be removed
    ;temp_data_lifetime = 24h

    # Directory where grafana can store logs
    ;logs = /var/log/grafana

    # Directory where grafana will automatically scan and look for plugins
    ;plugins = /var/lib/grafana/plugins

Please expect this section to be improved. Also, an important TODO concerns itself with writing a robust script that handles all
this automatically.

## Setup
Setting up the plugin can be accomplished just like with any other: you should simply add it through the web interface. The settings
one can configure are:

1. **Instance URI**: The [URI][] of the MongoDB instance to target.
1. **Database**: The name of the database containing the time series collections whose data we want to visualise. Bear in mind you'll need to
    create a new datasource for each database you want to query.
1. **Basic Auth**: This switch will allow you to define a username and password to authenticate with the MongoDB instance. Please bear in mind
    that even though the UI is ready this authentication **hasn't** been implemented yet... It's on the TODO list too!
1. **User**: The user to connect to the MongoDB instance as. Again, this will be silently ignored for now...
1. **Password**: The user's password. Once more, this will be silently ignored for now...
1. **Default editor mode**: Queries can be built by either writing the JSON payload 'as-is' or by using a (somewhat) interactive interface with
    some degree of data autodiscovery. This setting controls which UI you'll be presented with by default. The former way of specifying the payload
    is what we deem as the *Code* edit mode, whilst the latter is what we call the *Builder* edit mode.

Once that's ready you should hit the `Save & test` button to check everything works as expected. If it does you're good to go! From this point on
the datasource can be used just like any other. You just need to know the query format...

## The query format
The builder edit mode offers a great deal of flexibility and the chance to craft a truly handy interface for the user. The problem is it's very
easy to overload the backing MongoDB instance which a myriad queries when crafting the 'schema' of MongoDB's collections. After all, MongoDB's
somewhat 'unstructured' nature makes it rather hard to unify all the data in a collection in a known, simple schema. This is what motivated me
to allow the user to define a 'raw' [find() query][] (including its projection) to define what data to visualise.

This *find query* should be specified as a `string` resembling what you'd pass to `mongosh`. For instance, if we wanted to graph all the values
with a given `tag.host` we could specify:

    {"tags.host": "foobar"}

The *find query* will allow us to define what to filter on, but we still need to select what field from each document we want to graph. This should
be specified as a `string` too, but we don't need to add any additional formatting: the projection is generated internally. If we were to look for
the field `sampleField` in every document we would wind up with a payload such as:

```json
{
    "findQuery": "{\"tags.host\": \"foobar\"}",
    "projection": "sampleField"
}
```

The above can be inserted into the editor shown when working in the *Code* edit mode and it should be parsed without a problem. Please bear in
mind that, as a convenience, you can use `''` to define strings within the `findQuery` element even if that's not valid JSON. These single
quotes will be converted back to `""` internally so that you needn't escape all of them if writing the raw payload.

All the other necessary context for the query such as the *from* and *to* limits and so on is automatically populated by the plugin based on
the current settings.

## Developing
In order to develop the plugin one needs to have several dependencies installed:

1. **Golang**: The plugin's backend is written in Go so we need a local installation...
1. **NPM**: The frontend leverages React and that means whe need Node.js (I think, I honestly have no idea about the
    whole JavaScript ecosystem).
1. **Yarn**: Continuing with the whole JavaScript stuff we also need Yarn.
1. **Mage**: The building of the Go backend is automated with `mage` so we need to throw that in too.
1. **Make**: The building and bundling of the plugin is automated with `make` for now with a very rudimentary `Makefile`. Be sure to
    check it for information on available targets.

Please bear in mind that when getting set up the following need to be run:

    # Install JavaScript dependencies
    $ yarn install

Even though its a laughable bad practice we have been developing the plugin by bundling everything together, zipping it, uploading it to a
live Grafana server through `scp(1)`, installing it as explained above and erasing the caches on our browser. An example for Firefox can
be seen [here][https://support.mozilla.org/en-US/kb/how-clear-firefox-cache]. Bear in mind it's very handy to enable debug-level logging
just for `gtom` which can be done by adding the following to `/etc/grafana/grafana.ini`:

    [log]
    # Either "debug", "info", "warn", "error", "critical", default is "info"
    level = error

    # optional settings to set different levels for specific loggers. Ex filters = sqlstore:debug
    filters = plugin.gtom:debug

The above configuration will only show error-level logs for the rest of Grafana and debug-level logs for the plugin.

## Contributing
PRs and suggestions are more than welcome! Feel free to open an issue on PR.

## Further reading
Be sure to check [Grafana's plugin development documentation][] for a ton of really useful information. This includes:

1. [**Packaging a plugin**][]
1. [**Building a backend plugin**][]
1. [**MongoDB Go driver**][]
1. [**Convert a frontend plugin to a backend one**][]
1. [**Grafana UI Components**][]
1. [**Grafana UI Catalog**][]

<!-- REFS -->
[plugin policy]: https://grafana.com/legal/plugins/
[enterprise plugin]: https://grafana.com/docs/plugins/grafana-mongodb-datasource/latest/
[time series collections]: https://www.mongodb.com/docs/manual/core/timeseries-collections/
[simpod-json-datasource]: https://grafana.com/grafana/plugins/simpod-json-datasource/
[123RF Free Images]: https://www.123rf.com/free-images/
[captainvector]: https://www.123rf.com/profile_captainvector
[`grafana-mongodb-community-plugin`]: https://github.com/meln5674/grafana-mongodb-community-plugin
[URI]: https://www.mongodb.com/docs/manual/reference/connection-string/
[find() query]: https://www.mongodb.com/docs/manual/reference/method/db.collection.find/
[Grafana's plugin development documentation]: https://grafana.com/developers/plugin-tools/
[**Packaging a plugin**]: https://grafana.com/developers/plugin-tools/publish-a-plugin/package-a-plugin
[**Building a backend plugin**]: https://grafana.com/developers/plugin-tools/tutorials/build-a-data-source-backend-plugin
[**MongoDB Go driver**]: https://www.mongodb.com/docs/drivers/go/current/
[**Convert a frontend plugin to a backend one**]: https://grafana.com/developers/plugin-tools/how-to-guides/data-source-plugins/convert-a-frontend-datasource-to-backend
[**Grafana UI Components**]: https://github.com/grafana/grafana/tree/v10.4.2/packages/grafana-ui/src/components
[**Grafana UI Catalog**]: https://developers.grafana.com/ui/latest/index.html
