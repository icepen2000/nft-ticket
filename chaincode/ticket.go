package chaincode

import (
	"encoding/json"
	"fmt"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type NFTTicketChaincode struct {
	contractapi.Contract
}

// TransferRecord To store owners and timestamps
type TransferRecord struct {
	Owner     string `json:"old owner"`
	Timestamp string `json:"timestamp"`
}

// Ticket structure defines the data model
type Ticket struct {
	TicketID             string           `json:"ticketId"`
	EventID              string           `json:"eventId"`
	Owner                string           `json:"owner"`
	TicketPrice          int              `json:"ticketPrice"`
	Venue                string           `json:"venue"`
	EventDate            string           `json:"eventDate"`
	SeatNumber           string           `json:"seatNumber"`
	Revoked              bool             `json:"revoked"`
	MintedTimestamp      string           `json:"mintedTimestamp"`
	RevokedTimestamp     string           `json:"revokedTimestamp"`
	TransferHistory      []TransferRecord `json:"transferHistory"`
}

// CreateEvent creates a new event. Do I need this function?

// MintTicket issues a new ticket for an event
func (t *NFTTicketChaincode) MintTicket(ctx contractapi.TransactionContextInterface,
	ticketId string, eventId string, owner string, ticketPrice int, venue string, eventDate string, seatNumber string) error {

	exists, err := t.AssetExists(ctx, ticketId)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the asset %s already exists", ticketId)
	}

	// Get the transaction timestamp
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return err
	}

	formattedTimestamp, err := formatTimestamp(txTimestamp)
	if err != nil {
		return err
	}
	// Create a new ticket asset
	asset := Ticket{
		TicketID:        ticketId,
		EventID:         eventId,
		Owner:           owner,
		TicketPrice:     ticketPrice,
		Venue:           venue,
		EventDate:       eventDate,
		SeatNumber:      seatNumber,
		MintedTimestamp: formattedTimestamp,
		TransferHistory: []TransferRecord{},
	}

	// Serialize the ticket to JSON for storing in the ledger
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}

	// Save the ticket to the ledger
	return ctx.GetStub().PutState(ticketId, assetJSON)
}

// QueryTicket returns the asset stored in the world state with given id.
func (t *NFTTicketChaincode) QueryTicket(ctx contractapi.TransactionContextInterface, ticketId string) (*Ticket, error) {
	assetJSON, err := ctx.GetStub().GetState(ticketId)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if assetJSON == nil {
		return nil, fmt.Errorf("the asset %s does not exist", ticketId)
	}

	var asset Ticket
	err = json.Unmarshal(assetJSON, &asset)
	if err != nil {
		return nil, err
	}

	return &asset, nil
}

// TransferTicket transfers a ticket to a new owner
func (t *NFTTicketChaincode) TransferTicket(ctx contractapi.TransactionContextInterface, ticketId string, newOwner string) (string, error) {
	asset, err := t.QueryTicket(ctx, ticketId)
	if err != nil {
		return "", err
	}

	// Get the current transaction timestamp
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return "", err
	}
	formattedTimestamp, err := formatTimestamp(txTimestamp)
	if err != nil {
		return "", err
	}

	// Add the current owner and timestamp to the transfer history
	transferRecord := TransferRecord{
		Owner:     asset.Owner,
		Timestamp: formattedTimestamp,
	}
	asset.TransferHistory = append(asset.TransferHistory, transferRecord)

	// Update the ticket owner
	oldOwner := asset.Owner
	asset.Owner = newOwner

	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return "", err
	}

	err = ctx.GetStub().PutState(ticketId, assetJSON)
	if err != nil {
		return "", err
	}

	return oldOwner, nil
}

// QueryTicketsByEvent will retrieve all tickets for a specific event. Since Fabric doesn't support complex queries
// natively, we would typically use CouchDB as the state database to execute rich queries.
func (t *NFTTicketChaincode) QueryTicketsByEvent(ctx contractapi.TransactionContextInterface, eventId string) ([]*Ticket, error) {
	queryString := fmt.Sprintf(`{"selector":{"eventId":"%s"}}`, eventId)
	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var tickets []*Ticket
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		var ticket Ticket
		err = json.Unmarshal(queryResponse.Value, &ticket)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, &ticket)
	}
	return tickets, nil
}

func (t *NFTTicketChaincode) QueryTicketsByOwner(ctx contractapi.TransactionContextInterface, owner string) ([]*Ticket, error) {
	queryString := fmt.Sprintf(`{"selector":{"owner":"%s"}}`, owner)
	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var tickets []*Ticket
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		var ticket Ticket
		err = json.Unmarshal(queryResponse.Value, &ticket)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, &ticket)
	}
	return tickets, nil
}

func (t *NFTTicketChaincode) GetAllTickets(ctx contractapi.TransactionContextInterface) ([]*Ticket, error) {
	// range query with empty string for startKey and endKey does an
	// open-ended query of all assets in the chaincode namespace.
	resultsIterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var tickets []*Ticket
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var ticket Ticket
		err = json.Unmarshal(queryResponse.Value, &ticket)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, &ticket)
	}

	return tickets, nil
}
func (t *NFTTicketChaincode) RevokeTicket(ctx contractapi.TransactionContextInterface, ticketId string) error {
	ticket, err := t.QueryTicket(ctx, ticketId)
	if err != nil {
		return err
	}
	// Add a field in Ticket struct for revocation status, then set it here
	ticket.Revoked = true

	// Get the current transaction timestamp
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return err
	}

	// Format the timestamp (assuming you have a function formatTimestamp)
	formattedTimestamp, err := formatTimestamp(txTimestamp)
	if err != nil {
		return err
	}

	// Update the revocation timestamp in the ticket
	ticket.RevokedTimestamp = formattedTimestamp

	assetJSON, err := json.Marshal(ticket)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(ticketId, assetJSON)
}

// DeleteAllAssets I will remove this function later as revokeTicket should be just fine
func (t *NFTTicketChaincode) DeleteAllAssets(ctx contractapi.TransactionContextInterface) error {
	resultsIterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return err
	}
	defer resultsIterator.Close()

	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return err
		}

		err = ctx.GetStub().DelState(response.Key)
		if err != nil {
			return fmt.Errorf("failed to delete ticket %s: %v", response.Key, err)
		}
	}

	return nil
}

// AssetExists returns true when asset with given ID exists in world state
func (t *NFTTicketChaincode) AssetExists(ctx contractapi.TransactionContextInterface, ticketId string) (bool, error) {
	assetJSON, err := ctx.GetStub().GetState(ticketId)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}

	return assetJSON != nil, nil
}

// formatTimestamp converts a protobuf Timestamp to a human-readable string.
func formatTimestamp(ts *timestamppb.Timestamp) (string, error) {
	if ts == nil {
		return "", fmt.Errorf("provided timestamp is nil")
	}
	// Convert protobuf Timestamp to Go's time.Time
	t := ts.AsTime()
	// Format the time as string. The layout is based on the reference time:
	// Mon Jan 2 15:04:05 MST 2006
	return t.Format(time.RFC3339), nil
}

