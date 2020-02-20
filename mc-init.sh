#!/bin/sh

sleep 4 # Wait for minio to start up

mc config host add minio http://minio:9000 "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" --api s3v4

mc admin policy add minio read-create /read-create.json

mc admin user add minio mcbin-user abc0987654321

mc admin policy set minio read-create user=mcbin-user

mc mb -p minio/mcbin
