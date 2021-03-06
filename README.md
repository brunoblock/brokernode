# Brokernode

## Getting Started

The broker node uses Docker to spin up a go app, [mysql, required download](https://dev.mysql.com/downloads/file/?id=479845), and private iota instance (TODO). You must first install [Docker](https://www.docker.com/community-edition).

```bash
# To setup this first time, you need to have .env file. By default, use .env.test for unit test.
# Feel free to modify the .env file. Note: we don't check in .env file.
cp .env.test .env

# The database setup is different between dev and prod. Run the following script to change your DB setup.
# NOTE: Please do not check in the changes to your database.yml and docker-compose.yml file.
make docker-setup-dev
# make docker-setup-prod # use this to setup prod.

# Starts the brokernode on port 3000
DEBUG=1 docker-compose up --build -d # This takes a few minutes when you first run it.

# You only need to pass in --build the first time, or when you make a change to the container
# This uses cached images, so it's much faster to start.
DEBUG=1 docker-compose up -d

# Note, don't include `DEBUG=1` if you would like to run a production build.
# This will have less logs and no hot reloading.
docker-compose up --build -d
docker-compose up -d

# Executing commands in the app container
# Use `docker-compose exec YOUR_COMMAND`
# Eg: To run buffalo's test suite, run:
docker-compose exec app buffalo test

# Get a bash shell in the app container
docker-compose exec app bash

# Once in the app container, you can use all buffalo commands:
brokernode# buffalo db migrate
brokernode# buffalo test
```

---
#Prometheus Go client library

Monitoring and alerting toolkit
https://prometheus.io/docs/introduction/overview/

For new histogram use prepareHistogram() on init service services/prometheus.go
defer with histogramSeconds() and histogramData() on body other function

Using the expression browser UI http://localhost:9090/
Let us try looking at some data that Prometheus has collected about itself. To use Prometheus's built-in expression browser, navigate to http://localhost:9090/graph and choose the "Console" view within the "Graph" tab.

As you can gather from http://localhost:9090/metrics, one metric that Prometheus exports about itself is called http_requests_total (the total number of HTTP requests the Prometheus server has made). Go ahead and enter this into the expression console:

http_requests_total
This should return a number of different time series (along with the latest value recorded for each), all with the metric name http_requests_total, but with different labels. These labels designate different types of requests.

If we were only interested in requests that resulted in HTTP code 200, we could use this query to retrieve that information:

http_requests_total{code="200"}
To count the number of returned time series, you could write:

count(http_requests_total)
For more about the expression language, see the expression language documentation.

Using the graphing interface
To graph expressions, navigate to http://localhost:9090/graph and use the "Graph" tab.

For example, enter the following expression to graph the per-second HTTP request rate happening in the self-scraped Prometheus:

rate(http_requests_total[1m])

---

# Welcome to Buffalo!

Thank you for choosing Buffalo for your web development needs.

## Database Setup

It looks like you chose to set up your application using a mysql database! Fantastic!

The first thing you need to do is open up the "database.yml" file and edit it to use the correct usernames, passwords, hosts, etc... that are appropriate for your environment.

You will also need to make sure that **you** start/install the database of your choice. Buffalo **won't** install and start mysql for you.

### Create Your Databases

Ok, so you've edited the "database.yml" file and started mysql, now Buffalo can create the databases in that file for you:

    $ buffalo db create -a

## Starting the Application

Buffalo ships with a command that will watch your application and automatically rebuild the Go binary and any assets for you. To do that run the "buffalo dev" command:

    $ buffalo dev

If you point your browser to [http://127.0.0.1:3000](http://127.0.0.1:3000) you should see a "Welcome to Buffalo!" page.

**Congratulations!** You now have your Buffalo application up and running.

## What Next?

We recommend you heading over to [http://gobuffalo.io](http://gobuffalo.io) and reviewing all of the great documentation there.

Good luck!

[Powered by Buffalo](http://gobuffalo.io)

## Grafana
user: admin
password: admin
password can change on docker-compose.yml

## Config Datasource
[http://localhost:3100/datasources](http://localhost:3100/datasources)

Name: Monitor
Type: Prometheus
Url: [http://localhost:9090](http://localhost:9090)
Access: proxy

## Config Graph
[http://localhost:3100/dashboard/new](http://localhost:3100/dashboard/new)
Add panel - Graph - Monitor - Metrics

create query -> Metrics lookup -> prometheus_ || any_variable
Panel data source -> Monitor

## variable on metrics
treasures_verify_and_claim_seconds
upload_session_resource_create_seconds
upload_session_resource_update_seconds
upload_session_resource_create_beta_seconds
upload_session_resource_get_payment_status_seconds
webnode_resource_create_seconds
transaction_brokernode_resource_create_seconds
transaction_brokernode_resource_update_seconds
transaction_genesis_hash_resource_create_seconds
transaction_genesis_hash_resource_seconds
claim_unused_prls_seconds
claim_treasure_for_webnode_seconds
check_alpha_payments_seconds
check_beta_payments_seconds
flush_old_web_nodes_seconds
process_paid_sessions_seconds
update_msg_status_seconds
bury_treasure_addresses_seconds
process_unassigned_chunks_seconds
purge_completed_sessions_seconds
store_completed_genesis_hashes_seconds
remove_unpaid_upload_session_seconds
update_time_out_datamaps_seconds
verify_datamaps_seconds
