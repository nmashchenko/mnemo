package api

import (
	"encoding/base64"
	"github.com/skip2/go-qrcode"
	"mnemo/services/ws"
	"net/http"
)

type createRoomResponse struct {
	QRCode string `json:"qr_code"` // base64 PNG
}

func (a *API) createQRCodeHandler(wr http.ResponseWriter, r *http.Request) {

	// TODO: dynamically generate from config
	png, err := qrcode.Encode("localhost:8080/api/v1/join", qrcode.Medium, 256)
	if err != nil {
		WriteJSON(wr, ResponseJSON{Status: http.StatusInternalServerError, Message: "failed to generate QR"}, http.StatusInternalServerError)
		return
	}

	resp := createRoomResponse{
		QRCode: base64.StdEncoding.EncodeToString(png),
	}
	WriteJSON(wr, resp, http.StatusOK)
}

func (a *API) joinHubHandler(wr http.ResponseWriter, r *http.Request) {
	ws.ServeWs(a.deps.WebsocketHubService, wr, r)
}
