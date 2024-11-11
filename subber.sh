mosquitto_sub -h mqtt.home.arpa -p 1883 -v -t "/home/homely/#" | while read -r message; do
  echo "$(date '+%Y-%m-%d %H:%M:%S') - $message"
done

