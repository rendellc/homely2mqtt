package homely

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"time"

	gosocketio "github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
	"golang.org/x/oauth2/clientcredentials"
)

type Client struct {
	creds  *clientcredentials.Config
	client *http.Client
}

func (c *Client) createHttpClient(ctx context.Context) error {
	if c.client != nil {
		return nil
	}

	c.client = c.creds.Client(ctx)

	if c.client == nil {
		return fmt.Errorf("unable to create client")
	}
	return nil
}

func NewClient(username, password string) Client {
	return Client{
		creds: &clientcredentials.Config{
			TokenURL: "https://sdk.iotiliti.cloud/homely/oauth/token",
			EndpointParams: url.Values{
				"username": {username},
				"password": {password},
			},
		},
		client: nil,
	}
}

func (c *Client) GetLocation(ctx context.Context) (*Location, error) {
	err := c.createHttpClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetLocation failed to create client: %w", err)
	}

	res, err := c.client.Get("https://sdk.iotiliti.cloud/homely/locations")
	if err != nil {
		return nil, fmt.Errorf("unable to query locations: %w", err)
	}
	defer res.Body.Close()

	locations := []Location{}
	err = json.NewDecoder(res.Body).Decode(&locations)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal locations: %s", err.Error())
	}

	if len(locations) == 0 {
		log.Fatalf("no locations found")
		return nil, fmt.Errorf("no locations found")
	} else if len(locations) > 1 {
		return nil, fmt.Errorf("more than 1 location unsupported, found %d: %v", len(locations), locations)
	}

	location := locations[0]
	return &location, nil
}

func (c *Client) GetHome(ctx context.Context, locationID string) (*Home, error) {
	err := c.createHttpClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetHome failed to create client: %w", err)
	}

	homeURL := fmt.Sprintf("https://sdk.iotiliti.cloud/homely/home/%s", locationID)
	res, err := c.client.Get(homeURL)
	if err != nil {
		return nil, fmt.Errorf("unable to get home info: %w", err)
	}
	defer res.Body.Close()

	home := Home{}
	err = json.NewDecoder(res.Body).Decode(&home)
	if err != nil {
		return nil, fmt.Errorf("unable to decode body to Home: %w", err)
	}

	return &home, nil
}

// func getAlarmState(ctx context.Context, cred *clientcredentials.Config, home Home) error {
// 	client := cred.Client(ctx)
//
// 	// res, err := client.Get("https://sdk.iotiliti.cloud/homely/alarm/state")
// 	res, err := client.Get(fmt.Sprintf("https://sdk.iotiliti.cloud/homely/home/%s", home.LocationID))
// 	if err != nil {
// 		return fmt.Errorf("unable to get alarm state: %w", err)
// 	}
// 	defer res.Body.Close()
//
// 	body, err := io.ReadAll(res.Body)
// 	if err != nil {
// 		return fmt.Errorf("unable to read body: %w", err)
// 	}
//
// 	fmt.Printf("%s\n", string(body))
//
// 	return nil
// }

func (c *Client) ConnectHome(
	ctx context.Context,
	locationID string,
	deviceChangeCB func(EventDeviceStateChanged),
	alarmChangeCB func(EventAlarmStateChanged),
) error {
	log.Printf("connect to home %s", locationID)

	token, err := c.creds.Token(ctx)
	if err != nil {
		return fmt.Errorf("unable to create tokens: %w", err)
	}

	t := transport.GetDefaultWebsocketTransport()
	t.PingInterval = 30 * time.Second

	done := make(chan error, 1)

	conn, err := gosocketio.Dial(
		fmt.Sprintf("%s&locationId=%s&token=Bearer%%20%s",
			gosocketio.GetUrl("sdk.iotiliti.cloud", 443, true),
			locationID, token.AccessToken),
		t,
	)
	if err != nil {
		return fmt.Errorf("unable to dial homely socket io api: %w", err)
	}
	defer conn.Close()

	err = conn.On(gosocketio.OnConnection, func(c *gosocketio.Channel) {
		log.Println("Connected")
	})
	if err != nil {
		return fmt.Errorf("unable to add OnConnection handler: %w", err)
	}

	err = conn.On(gosocketio.OnError, func(c *gosocketio.Channel) {
		log.Printf("error: %v", c)
		done <- fmt.Errorf("socketio error: %v", c)
	})
	if err != nil {
		return fmt.Errorf("unable to add OnError handler: %w", err)
	}
	err = conn.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
		log.Println("Disconnected")
		done <- fmt.Errorf("disconnected")
	})
	if err != nil {
		return fmt.Errorf("unable to add OnDisconnection handler: %w", err)
	}

	err = conn.On("event", func(c *gosocketio.Channel, ev any) {
		event, ok := ev.(map[string]any)
		if !ok {
			done <- fmt.Errorf("unexpected event type: %v, %v", reflect.TypeOf(ev), ev)
			return
		}

		switch event["type"] {
		case "device-state-changed":
			stateChangeEvent, err := parseDeviceStateChangeEvent(ev)
			if err != nil {
				done <- fmt.Errorf("unable to parse device-state-changed event: %w", err)
				return
			}
			log.Printf("calling device-state-change handler: %v", stateChangeEvent)
			deviceChangeCB(stateChangeEvent)
		case "alarm-state-changed":
			alarmStateChangeEvent, err := parseAlarmStateChangeEvent(ev)
			if err != nil {
				done <- fmt.Errorf("unable to parse alarm-state-changed event: %w", err)
				return
			}
			log.Printf("calling alarm-state-change handler: %v", alarmStateChangeEvent)
			alarmChangeCB(alarmStateChangeEvent)

		default:
			done <- fmt.Errorf("unhandled event type: %v, %v", event["type"], event)

		}
	})
	if err != nil {
		return fmt.Errorf("unable to add event handler: %w", err)
	}

	select {
	case err := <-done:
		if err != nil {
			log.Printf("home connection error: %s", err.Error())
			return fmt.Errorf("home connection error: %w", err)
		}

		log.Printf("connection to home %s closed", locationID)
		return nil
	case <-ctx.Done():
		log.Printf("connect home context done")
		return ctx.Err()
	}

}
