package main

import (
	"flag"
	"fmt"
	"github.com/amimof/huego"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
)

var (
	bridgeUrl   string
	username    string
	listenAddr  string
	metricsPath string
	upDesc      prometheus.Desc
	statusDesc  prometheus.Desc
)

type Exporter struct {
	bridge huego.Bridge
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- &upDesc
	ch <- &statusDesc
}

func stateMetric(light *huego.Light) prometheus.Metric {
	lightID := fmt.Sprint(light.ID)
	lightOn := float64(0)
	if light.State.On {
		lightOn = 1
	}
	lightReachable := "0"
	if light.State.Reachable {
		lightReachable = "1"
	}
	lightBrightness := fmt.Sprint(light.State.Bri)
	lightHue := fmt.Sprint(light.State.Hue)
	lightSaturation := fmt.Sprint(light.State.Sat)

	return prometheus.MustNewConstMetric(
		&statusDesc,
		prometheus.GaugeValue,
		lightOn,
		light.Name, lightID, light.UniqueID, lightReachable, lightBrightness, lightHue,
		lightSaturation,
	)

}

func upMetric(up float64) prometheus.Metric {

	return prometheus.MustNewConstMetric(
		&upDesc,
		prometheus.GaugeValue,
		up,
	)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	lights, err := e.bridge.GetLights()
	if err != nil {
		log.Printf("Error getting lights: %s", err)
		ch <- upMetric(0)
		return
	}
	log.Printf("Found %d lights", len(lights))
	ch <- upMetric(1)

	for _, light := range lights {
		ch <- stateMetric(&light)
	}
}

func NewExporter(bridge *huego.Bridge) (*Exporter, error) {
	// attempt to GetLights() once in case there's an error
	_, err := bridge.GetLights()
	return &Exporter{
		bridge: *bridge,
	}, err
}

func indexHandler(metricsPath string) http.HandlerFunc {
	html := `
<html>
	<head>
		<title>Hue Exporter</title>
	</head>
	<body>
		<h1>Hue Exporter</h1>
		<p>
			<a href='%s'>metrics</a>
		</p>
	</body>
</html>
`

	index := []byte(fmt.Sprintf(html, metricsPath))

	return func(w http.ResponseWriter, r *http.Request) {
		w.Write(index)
	}
}

func init() {
	flag.StringVar(&bridgeUrl, "bridge-url", "http://philips-hue", "url of the Hue bridge device")
	flag.StringVar(&username, "username", "", "set authorised API username")
	flag.StringVar(&listenAddr, "listen-address", ":9100", "set HTTP listen address")
	flag.StringVar(&metricsPath, "metrics-path", "/metrics", "set metrics path")

	// Desc
	upDesc = *prometheus.NewDesc("up", "Was the last query successful?", []string{}, prometheus.Labels{})
	statusDesc = *prometheus.NewDesc("light_status", "State of light (on/off)",
		[]string{"name", "id", "unique_id", "reachable", "brightness", "hue", "saturation"},
		prometheus.Labels{})
}

// Register with curl command:
// $ curl -X POST -d '{"devicetype":"my app name#my_username"}' http://philips-hue/api
// [{"error":{"type":101,"address":"","description":"link button not pressed"}}]
// Then press huge bridge link button and run curl again:
// $ curl -X POST -d '{"devicetype":"my app name#my_username"}' http://philips-hue/api
// [{"success":{"username":"VBYPKZXBqwcLCSdzj5yLW1gjK2fb9XCOSxQ1dP7B"}}]
// Username is: VBYPKZXBqwcLCSdzj5yLW1gjK2fb9XCOSxQ1dP7B

func main() {
	flag.Parse()

	if username == "" {
		fmt.Println("Please set a username")
		os.Exit(0)
	}

	bridge := huego.New(bridgeUrl, username)
	exporter, err := NewExporter(bridge)
	if err != nil {
		log.Fatal(err)
	}

	prometheus.MustRegister(exporter)

	http.HandleFunc("/", indexHandler(metricsPath))
	http.Handle(metricsPath, promhttp.Handler())
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
