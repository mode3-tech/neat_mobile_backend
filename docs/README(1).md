# Transfer Service Documentation

## 1. Overview

The Transfer Service supports two types of bank transfers:

1. **Intrabank Transfer** — transfers between Optimus Bank accounts.
2. **Interbank Transfer** — transfers from an Optimus Bank account to an account held at another bank.

For both transfer types, the **source account must first be created through the appropriate account-opening APIs** before initiating a transfer.

---

## 2. Transfer Flow

The general transfer flow is:

1. Create the source account using the appropriate account-opening API.
2. Retrieve the list of supported banks using the **Get Bank Details** endpoint.
3. Select the beneficiary's bank.
4. Determine the transfer type:
   - If **Optimus Bank** is selected → **Intrabank Transfer**
   - If any other bank is selected → **Interbank Transfer**
5. Perform the required beneficiary name enquiry.
6. Initiate the transfer using the details required by the applicable flow.

---

## 3. Get Bank Details

Before initiating a transfer, call the **Get Bank Details** endpoint to retrieve the list of supported banks.

The response provides the bank information required to identify the beneficiary's bank, including the **bank code**.

The beneficiary's bank code should be obtained from this endpoint rather than being manually entered.

### Determining the Transfer Type

After retrieving the bank list:

- **Optimus Bank selected** → initiate an **Intrabank Transfer**.
- **Any other bank selected** → initiate an **Interbank Transfer**.

---

# 4. Intrabank Transfer

An intrabank transfer is a transfer from an Optimus Bank account to another **Optimus Bank account**.

## 4.1 Name Enquiry

A **name enquiry must be performed** against the beneficiary account before initiating the transfer.

The beneficiary account number and the Optimus Bank bank code are used to perform the enquiry.

## 4.2 Session ID

For an intrabank transfer, the **Session ID should be left empty**.

Unlike an interbank transfer, an intrabank transfer does not require the Session ID returned from the name-enquiry service.

## 4.3 Required Transfer Fields

| Field | Description | Required |
|---|---|---|
| `requestId` | Unique identifier for the transfer request. A GUID should be generated for each request. | Yes |
| `transactionReference` | Reference value used to identify and reference the payment/transfer transaction. | Yes |
| `narration` | Description or purpose of the transaction. | Yes |
| `beneficiaryAccount` | Destination/beneficiary account number. | Yes |
| `beneficiaryBankCode` | Bank code of the beneficiary's bank. For an intrabank transfer, this should be the Optimus Bank code obtained from the Get Bank Details endpoint. | Yes |
| `sessionId` | Session identifier from name enquiry. Not required for intrabank transfers. | No / Empty |

## 4.4 Intrabank Flow

```text
Create Source Account
        ↓
Get Bank Details
        ↓
Select Optimus Bank
        ↓
Perform Beneficiary Name Enquiry
        ↓
Initiate Intrabank Transfer
        ↓
Transfer Completed
```

---

# 5. Interbank Transfer

An interbank transfer is a transfer from an Optimus Bank account to an account held at **another bank**.

## 5.1 Name Enquiry

A **name enquiry must be performed** against the beneficiary account before initiating the transfer.

The name-enquiry response provides the **Session ID** required to initiate the interbank transfer.

## 5.2 Session ID

For an interbank transfer, the **Session ID is compulsory**.

The Session ID must be obtained from the **Name Enquiry** endpoint and supplied when initiating the interbank transfer.

Do not generate or manually provide a Session ID.

## 5.3 Required Transfer Fields

| Field | Description | Required |
|---|---|---|
| `requestId` | Unique identifier for the transfer request. A GUID should be generated for each request. | Yes |
| `transactionReference` | Reference value used to identify and reference the payment/transfer transaction. | Yes |
| `narration` | Description or purpose of the transaction. | Yes |
| `beneficiaryAccount` | Destination/beneficiary account number. | Yes |
| `beneficiaryBankCode` | Bank code of the beneficiary's bank, obtained from the Get Bank Details endpoint. | Yes |
| `sessionId` | Session ID returned by the Name Enquiry endpoint. | **Yes** |

## 5.4 Interbank Flow

```text
Create Source Account
        ↓
Get Bank Details
        ↓
Select Beneficiary Bank
        ↓
Perform Beneficiary Name Enquiry
        ↓
Retrieve Session ID
        ↓
Initiate Interbank Transfer
        ↓
Transfer Completed
```

---

# 6. Key Difference Between Intrabank and Interbank Transfers

| Item | Intrabank | Interbank |
|---|---|---|
| Destination | Optimus Bank | Other banks |
| Bank Code | Optimus Bank code | Beneficiary bank code |
| Name Enquiry | Required | Required |
| Session ID | **Leave empty** | **Required** |
| Request ID | Required | Required |
| Transaction Reference | Required | Required |
| Narration | Required | Required |
| Beneficiary Account | Required | Required |

## Important

The primary distinction is the **beneficiary bank** and the handling of the **Session ID**:

- **Optimus → Optimus:** `sessionId` must be empty.
- **Optimus → Other Bank:** `sessionId` must be populated with the value returned by the Name Enquiry endpoint.
