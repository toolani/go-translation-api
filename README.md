# Translation API

A server providing an JSON HTTP API for managing translations of arbitrary strings between languages. Translations are stored in either a PostgreSQL or SQLite database and can be exported to XLIFF files on disk.

The Translation API was envisaged as a replacement for the web interface provided by the [JMSTranslationBundle][jms] and, as such, the XLIFF files it produces are fully compatible with the [Symfony][symfony] framework (as of at least Symfony v2.8).

A web user interface for the API provided by this project can be found in the [Translation Interface][translation-interface] repository.

[jms]: http://jmsyst.com/bundles/JMSTranslationBundle
[symfony]: https://symfony.com/

- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [API Specification](#api-specification)

## Requirements

### Recommended

- A PostgreSQL database - SQLite is also supported and provides identical functionality. The `init-db` command will initialise your chosen database with the required table structure.

## Installation

The `go-translation-api` is a standalone executable written in [Go][golang]. To build the executable simply clone this repository and build the project as you would any other Go project. i.e. check it out into the appropriate place under `src` on your `GOPATH` and run `go install` from the project's root directory.

Once you have the tool built, it requires a config file in order to tell it where its database is located and where XLIFF files should be read from and written to. An example config file for a PostgeSQL database would look like this:

```toml
[database]
driver = "postgres"
host = "localhost"
name = "database_name"
user = "database_user"
password = "sekr3t"

[server]
# The HTTP server will listen on this port
port = 8181

[xliff]
# When populating the database with translations imported from XLIFF files
# they will be read from this path
import_path = "/var/somepath/translations"
# New XLIFF files created by the Translation API will be created in this path
export_path = "/var/somepath/translations"
```

Or if using a SQLite database:

```toml
[database]
driver = "sqlite3"
file = "/var/some_path/translations.db"

[server]
# The HTTP server will listen on this port
port = 8181

[xliff]
# When populating the database with translations imported from XLIFF files
# they will be read from this path
import_path = "/var/somepath/translations"
# New XLIFF files created by the Translation API will be created in this path
export_path = "/var/somepath/translations"
```

When used together with a Symfony application, it is recommended that both the `xliff.import_path` and `xliff.export_path` are pointed at your development environment's translations directory. e.g. `/var/your_path/src/FooInc/SomeBundle/Resources/translations`.

By default the config file is expected to be in the current working directory, but this path can be overridden using the `-config` option.

[golang]: https://golang.org

## Usage

The Translation API is a command line executable that accepts one of a selection of commands controlling what it should do. This section will first provide a run through of a typical usage scenario, and then provide an overview of each of the available commands.

### Getting Started
#### Initialise the database
Assuming a config file is in place and the database it points to exists (if using PostgreSQL), first use the `init-db` command to initialise the database.

Note that all of the example commands in this section assume that the config file is in the same directory as the `go-translation-api` executable. Use the `-config` option if this is not the case.

```sh
$ ./go-translation-api init-db
```

#### Import existing translations
If you have existing translations in XLIFF files that you would like to manage using the Translation API, point the config file's `xliff.import_path` at their location and use the `import` command to import them into the database.

```sh
$ ./go-translation-api import
```

It is expected that XLIFF files to be imported in this way are named after the 'translation domain' and language that they contain translations for. Filenames are expected to conform to the pattern: `[domain].[language_code].xliff`. For example, a file containing English translations for the 'homepage' domain would be named `homepage.en.xliff` while a file containing Swiss German translations for the 'help' domain would be named `help.de-ch.xliff`.

#### Start the HTTP server
To start the API server, use the `serve` command. The server will listen on the port defined in the config file.

```sh
$ ./go-translation-api serve
```

To test whether it is working, this command can be used (here we are assuming the server is using port 8181).

```sh
$ curl http://localhost:8181/languages
```

This should produce some JSON output containing a list of languages, similar to this:

```json
[{"code":"de","name":"German"},{"code":"de-at","name":"German (Austria)"},{"code":"de-be","name":"German (Belgium)"},{"code":"de-ch","name":"German (Switzerland)"},{"code":"de-de","name":"German (Germany)"},{"code":"en","name":"English"}]
```

At this point the API server is up and running and you may want to set up the [Translation Interface][translation-interface] project for interacting with it.

### Command overview

When running `go-translation-api`, these commands are available:

#### init-db
Creates or updates the required database table structure for the Translation API.

Must be run at least once before any of the other commands.

No action is taken if the database is already up to date, that is, it is safe to run this command more than once, no translation data will be lost.

#### remove-db
Removes all tables created by the Translation API from the database.

All Translation API data will be deleted from the database.

Requires that the `-force` option is provided or nothing will happen.

#### serve
Starts the Translation API HTTP server using the settings defined in the config file.

Any changes to translations via the HTTP API will cause the related XLIFF files to be re-exported immediately after the change is successfully committed to the database.

#### import
Imports the content of the XLIFF files from the config file's `xliff.import_path` into the database. See the notes above regarding the expected file naming convention.

#### export
Exports translations from the database to XLIFF files in the config file's `xliff.export_path`.

As noted under the `serve` command, under normal usage - where changes to translation data are made exclusively via the HTTP API - the XLIFF files are automatically kept up to date with any translation changes. As such, this command is likely to mostly be useful in cases where the translation data has been edited directly in the database (and not via the HTTP API).

#### help
Prints usage instructions.

## API Specification
### Concepts 
This section attempts to explain the concepts and naming conventions used by the Translation API.

A 'Language' is a named entity represented by a code. For example, the language English is represented by the code `en`, German by `de`, English as used in America by `en-us`, German as used in Austria by `de-at`, and so on. A number of default languages are provided by the Translation API.

A 'Translation' is a string containing some content in a particular Language.

A 'String' is a named entity that we desire to provide translations for. A String may contain zero or more Translations, each into a different language. For example, our homepage may have a welcome message that we identify by the name `welcome_message`, our help page may have a form button containing a label that we identify by the name `submit_label`.

A translation 'Domain' is a named collection of Strings and their associated Translations. For example, all Strings for our homepage may be contained in a Domain called `homepage`.

When exporting data from the Translation API, the translation data is exported into XLIFF files with one file for each domain/language combination. For example, if our database contains a single domain `homepage` and this contains Strings that are only translated into English and French, an export would produce the files: `homepage.en.xliff` and `homepage.fr.xliff`.

### Endpoints
#### Domain index

```
GET /domains
```

Lists the names of all available Domains.

```json
{
  "domains": [
    "contact",
    "help",
    "homepage"
  ]
}
```

#### Get domain contents

```
GET /domains/{domain_name}
```

Gets all of the Strings belonging to a Domain and their Translations.

```json
{
  "name": "homepage",
  "strings": [
    {
      "name": "welcome",
      "translations": {
        "de": {
          "content": "Willkommen!"
        },
        "en": {
          "content": "Welcome!"
        }
      }
    }
  ]
}
```

#### Export domain to XLIFF

```
POST /domains/{domain_name}/export
```

Exports the contents of a Domain to XLIFF files.

Note that under normal operation, this endpoint should not be needed, as any translation modifications made via the API will automatically trigger a re-export of the affected XLIFF files.

As such, this endpoint would generally only be required if changes have been made directly to the translation data in the database (and not via this API).

```json
{
  "result": "ok"
}
```

#### Language index

```
GET /languages
```

Gets all of the available Languages.

```json
[
  {
    "code": "de",
    "name": "German"
  },
  {
    "code": "de-at",
    "name": "German (Austria)"
  },
  {
    "code": "en",
    "name": "English"
  }
]
```

#### Add a language

```
POST /languages/{language_code}
```

Adds a new language.

The request's body should be a JSON object with a `name` property containing the new language's display name as a string.

```json
{
  "result": "ok"
}
```

#### Delete a string

```
DELETE /domains/{domain_name}/strings/{string_name}
```

Deletes a String and all of its associated Translations.

```json
{
  "result": "ok"
}
```

#### Delete a translation

```
DELETE /domains/{domain_name}/strings/{string_name}/translations/{language_code}
```

Deletes the specified Translation of a String. Other translations are unaffected.

```json
{
  "result": "ok"
}
```

#### Create or update a translation (and create a string)

```
POST /domains/{domain_name}/strings/{string_name}/translations/{language_code}
PUT /domains/{domain_name}/strings/{string_name}/translations/{language_code}
```

When sent as a POST request, allows a new String to be created along with its first Translation, or allows a new Translation to be added for an existing String.

When sent as a PUT request allows an existing Translation to be updated.

The request's body should be a JSON object with a `content` property containing the Translation's desired content in the target Language.

```json
{
  "result": "ok"
}
```

#### Search for a string

```
GET /search
```

Can be used to search for Strings by either the String name or the Translation content.

Requires the query parameter `term` which should contain the text to search for.

Accepts an optional query parameter `by` which can be set to one of: `all`, `string_name`, `translation_content`. The default is `all`.

```json
[
  {
    "domain_name": "homepage",
    "string_name": "welcome",
    "language_code": "en",
    "translation_content": "Welcome!"
  },
  {
    "domain_name": "homepage",
    "string_name": "welcome",
    "language_code": "de",
    "translation_content": "Willkommen!"
  }
]
```

[translation-interface]: https://github.com/toolani/translation-interface