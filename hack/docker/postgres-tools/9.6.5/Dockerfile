FROM kubedb/postgres:9.6.5

COPY osm /usr/local/bin/osm
COPY postgres-tools.sh /usr/local/bin/postgres-tools.sh

ENTRYPOINT ["postgres-tools.sh"]
