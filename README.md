## FerretDB

- 2025-05-15: FerretDB 2.0 PostgreSQL testing:
  - https://forums.meteor.com/t/guide-how-to-use-meteor-with-postgres/55709/20
  - https://atmospherejs.com/meteorisgreat/ferretdb-index-fix
  - https://github.com/tannerlab/ferretdb-index-fix
- Meteor Forum post https://forums.meteor.com/t/ferretdb-1-18-now-has-oplog-support-trying-replace-mongodb-6-x-with-ferretdb-postgresql-or-ferretdb-sqlite/61092/4
- FerretDB known differences: https://docs.ferretdb.io/diff/
- FerretDB OpLog Support: https://docs.ferretdb.io/configuration/oplog-support/
  - https://github.com/FerretDB/FerretDB/issues/76
- FerretDB: Achieve compatibility with WeKan
  - https://github.com/FerretDB/FerretDB/issues/1752
- FerretDB: Improve compatibility with real specific apps and libraries
  - https://github.com/FerretDB/FerretDB/issues/5 
- FerretDB: High level Roadmap to end of Q4 2024:
  - https://github.com/orgs/FerretDB/projects/2/views/1
  - There is "No Stage" mention of Change Streams as a wish sometime:
    - https://github.com/FerretDB/FerretDB/issues/175

#### Optional: Deleting all Docker volumes etc data to cleanup

```
git clone https://github.com/wekan/docker-cleanup-volumes

cd docker-cleanup-volumes

./start.sh
```

#### 1) Starting this FerretDB/SQLite

```
git clone https://github.com/wekan/ferretdb

cd ferretdb/sqlite

docker compose up -d
```

#### 2) Manually create OpLog collection

```
mongosh

use local

db.createCollection('oplog.rs', { capped: true, size: 536870912 })
```

#### 3) Build WeKan

```
git clone https://github.com/wekan/wekan

cd wekan

./rebuild-wekan.sh

1

./rebuild-wekan.sh

2

./rebuild-wekan.sh

3
```

#### 4) OpLog Settings

These are at ~/repos/ferretdb/wekan/start-wekan.sh
```
cd ~/repos/wekan/.build/bundle

export MONGO_OPLOG_URL=mongodb://127.0.0.1:27017/local?replicaSet=rs0

export MONGO_URL=mongodb://127.0.0.1:27017/wekan

export ROOT_URL=http://localhost:4000

export PORT=4000

node main.js

cd ~/repos/ferretdb/wekan
```

#### 5) Start WeKan bundle

```
cd ~/repos/ferretdb/wekan

./start-wekan.sh
```
Web browser URL: http://localhost:4000

#### 6) Look that there is something at OpLog

```
mongosh

use local

db.oplog.rs.countDocuments()

db.oplog.rs.find()
```

#### 7) Look what has been able to write to database

```
mongosh

use wekan

show collections

db.boards.countDocuments()

db.boards.find()

db.cards.find()
```

#### 8) Currently working with FerretDB

- Registering new user at https://localhost:2000/sign-up
- Login at https://localhost:2000/sign-in
- Add new user at right top username / Admin Panel / People / People

#### 9) Currently not working with FerretDB

- Adding attachments related collections to GridFS
- Adding new board, errors about matchElement and userId

#### TODO: Later, try to fix this to create OpLog collection at start for PostgreSQL

docker-compose.yml
```
ferretdb-sqlite-setup:
 image: ghcr.io/ferretdb/all-in-one
 restart: no
 depends_on:
   - ferretdb-sqlite
 entrypoint: [ "bash", "-c", "sleep 10 && /usr/bin/mongosh --host ferretdb-postgresql:27017 --eval 'use local;db.createCollection('oplog.rs', { capped: true, size: 536870912 });'"]
```
