package wema

import (
	"context"
	"testing"

	"neat_mobile_app_backend/providers/baas"
)

func TestTransferAdapter_WemaVASClientRequests(t *testing.T) {
	f, alat := newFakeALAT(t, map[string]string{
		"/airtime-data/api/Airtime/Client/PurchaseAirtime": `{"result":{"status":"Successful","message":"done","transactionReference":"tx-airtime","platformTransactionReference":"alat-1"},"hasError":false}`,
		"/airtime-data/api/Data/Client/PurchaseData":       `{"result":{"status":"Successful","message":"done","transactionReference":"tx-data"},"hasError":false}`,
		"/bills-payment/api/Shared/PayBill":                `{"result":{"status":"Successful","message":"done","transactionReference":"tx-bill","value":"token"},"hasError":false}`,
		"/bills-payment/api/BillsPayment/ValidateCustomer": `{"result":{"isValidated":true,"customerName":"Ada"},"hasError":false}`,
		"/bills-payment/api/BillsPayment/GetAllBills":      `{"result":[{"id":2,"name":"Electricity","billers":[{"id":8,"name":"Power","identifier":"power","isAquired":true,"requiredValidation":true,"charge":50,"packages":[{"id":11,"billerId":8,"name":"Prepaid","amount":1000,"isAmountEditable":true}]}]}],"hasError":false}`,
		"/airtime-data/api/Data/GetDataPlans":              `{"result":[{"id":3,"networkProvider":"MTN","dataPackages":[{"id":17,"name":"1GB","amount":1000,"dataPlan":"1GB","validity_Period":"30 days"}]}],"hasError":false}`,
	})
	signer := NewSigner("secret")
	adapter := NewTransferAdapter(alat, signer)

	if _, err := adapter.PurchaseAirtime(context.Background(), "0123456789", "MTN", "08030000000", 100, "tx-airtime"); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.PurchaseData(context.Background(), "0123456789", "MTN", "08030000000", 17, 1000, "tx-data"); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.PayBill(context.Background(), "0123456789", "customer-1", "", "08030000000", "Ada", 11, 1000, 50, "tx-bill"); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.ValidateBill(context.Background(), "0123456789", 11); err != nil {
		t.Fatal(err)
	}

	for path, reference := range map[string]string{
		"/airtime-data/api/Airtime/Client/PurchaseAirtime": "tx-airtime",
		"/airtime-data/api/Data/Client/PurchaseData":       "tx-data",
		"/bills-payment/api/Shared/PayBill":                "tx-bill",
	} {
		call := f.call(path)
		if call == nil {
			t.Fatalf("expected request to %s", path)
		}
		if call.body["accountNumber"] != "0123456789" && call.body["customerAccount"] != "0123456789" {
			t.Errorf("%s request did not use the customer's Wema account: %+v", path, call.body)
		}
		var securityInfo string
		if path == "/bills-payment/api/Shared/PayBill" {
			securityInfo, _ = call.body["securityInfo"].(string)
		} else {
			securityInfo, _ = call.body["securityInfo"].(string)
		}
		if !signer.Verify(PurposeSecurityInfo, reference, securityInfo) {
			t.Errorf("%s securityInfo does not verify for %s", path, reference)
		}
	}

	bills, err := adapter.GetBills(context.Background())
	if err != nil || len(bills.Result) != 1 || bills.Result[0].Billers[0].Packages[0].ID != 11 {
		t.Fatalf("GetBills = %+v, %v", bills, err)
	}
	plans, err := adapter.GetDataPlans(context.Background())
	if err != nil || plans.Result[0].DataPackages[0].ID != 17 {
		t.Fatalf("GetDataPlans = %+v, %v", plans, err)
	}
}

func TestAlatClientVASResponseError(t *testing.T) {
	_, alat := newFakeALAT(t, map[string]string{
		"/airtime-data/api/Airtime/Client/PurchaseAirtime": `{"errorMessage":"invalid account","hasError":true}`,
	})
	adapter := NewTransferAdapter(alat, NewSigner("secret"))
	if _, err := adapter.PurchaseAirtime(context.Background(), "0123456789", "MTN", "08030000000", 100, "tx-1"); err == nil {
		t.Fatal("expected ALAT's VAS error to be returned")
	}
	if err := baas.AlatResultError(true, "", nil); err == nil {
		t.Fatal("expected a fallback error when ALAT omits the message")
	}
}
