package paypal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// APIBaseSandBox points to the sandbox (for testing) version of the API
	APIBaseSandBox = "https://api.sandbox.paypal.com"

	// APIBaseLive points to the live version of the API
	APIBaseLive = "https://api.paypal.com"

	// RequestNewTokenBeforeExpiresIn is used by SendWithAuth and try to get new Token when it's about to expire
	RequestNewTokenBeforeExpiresIn = time.Duration(60) * time.Second
)

// Possible values for `no_shipping` in InputFields
//
// https://developer.paypal.com/docs/api/payment-experience/#definition-input_fields
const (
	NoShippingDisplay      uint = 0
	NoShippingHide         uint = 1
	NoShippingBuyerAccount uint = 2
)

// Possible values for `address_override` in InputFields
//
// https://developer.paypal.com/docs/api/payment-experience/#definition-input_fields
const (
	AddrOverrideFromFile uint = 0
	AddrOverrideFromCall uint = 1
)

// Possible values for `landing_page_type` in FlowConfig
//
// https://developer.paypal.com/docs/api/payment-experience/#definition-flow_config
const (
	LandingPageTypeBilling string = "Billing"
	LandingPageTypeLogin   string = "Login"
)

// Possible value for `allowed_payment_method` in PaymentOptions
//
// https://developer.paypal.com/docs/api/payments/#definition-payment_options
const (
	AllowedPaymentUnrestricted         string = "UNRESTRICTED"
	AllowedPaymentInstantFundingSource string = "INSTANT_FUNDING_SOURCE"
	AllowedPaymentImmediatePay         string = "IMMEDIATE_PAY"
)

// Possible value for `intent` in CreateOrder
//
// https://developer.paypal.com/docs/api/orders/v2/#orders_create
const (
	OrderIntentCapture   string = "CAPTURE"
	OrderIntentAuthorize string = "AUTHORIZE"
)

// Possible values for `category` in Item
//
// https://developer.paypal.com/docs/api/orders/v2/#definition-item
const (
	ItemCategoryDigitalGood  string = "DIGITAL_GOODS"
	ItemCategoryPhysicalGood string = "PHYSICAL_GOODS"
)

// Possible values for `shipping_preference` in ApplicationContext
//
// https://developer.paypal.com/docs/api/orders/v2/#definition-application_context
const (
	ShippingPreferenceGetFromFile        string = "GET_FROM_FILE"
	ShippingPreferenceNoShipping         string = "NO_SHIPPING"
	ShippingPreferenceSetProvidedAddress string = "SET_PROVIDED_ADDRESS"
)

const (
	EventPaymentCaptureCompleted       string = "PAYMENT.CAPTURE.COMPLETED"
	EventPaymentCaptureDenied          string = "PAYMENT.CAPTURE.DENIED"
	EventPaymentCaptureRefunded        string = "PAYMENT.CAPTURE.REFUNDED"
	EventMerchantOnboardingCompleted   string = "MERCHANT.ONBOARDING.COMPLETED"
	EventMerchantPartnerConsentRevoked string = "MERCHANT.PARTNER-CONSENT.REVOKED"
)

const (
	OperationAPIIntegration   string = "API_INTEGRATION"
	ProductExpressCheckout    string = "EXPRESS_CHECKOUT"
	IntegrationMethodPayPal   string = "PAYPAL"
	IntegrationTypeThirdParty string = "THIRD_PARTY"
	ConsentShareData          string = "SHARE_DATA_CONSENT"
)

const (
	FeaturePayment               string = "PAYMENT"
	FeatureRefund                string = "REFUND"
	FeatureFuturePayment         string = "FUTURE_PAYMENT"
	FeatureDirectPayment         string = "DIRECT_PAYMENT"
	FeaturePartnerFee            string = "PARTNER_FEE"
	FeatureDelayFunds            string = "DELAY_FUNDS_DISBURSEMENT"
	FeatureReadSellerDispute     string = "READ_SELLER_DISPUTE"
	FeatureUpdateSellerDispute   string = "UPDATE_SELLER_DISPUTE"
	FeatureDisputeReadBuyer      string = "DISPUTE_READ_BUYER"
	FeatureUpdateCustomerDispute string = "UPDATE_CUSTOMER_DISPUTES"
)

const (
	LinkRelSelf      string = "self"
	LinkRelActionURL string = "action_url"
)

// Possible values for Operation in PatchObject
const (
	Add     string = "add"
	Replace string = "replace"
	Remove  string = "remove"
)

// Possible values for Path in PatchObject
const (
	Description string = "/description"
	Category    string = "/category"
	ImageUrl    string = "/image_url"
	HomeUrl     string = "/home_url"
)

type (
	// JSONTime overrides MarshalJson method to format in ISO8601
	JSONTime time.Time

	// Address struct
	Address struct {
		Line1       string `json:"line1"`
		Line2       string `json:"line2, omitempty"`
		City        string `json:"city"`
		CountryCode string `json:"country_code"`
		PostalCode  string `json:"postal_code, omitempty"`
		State       string `json:"state, omitempty"`
		Phone       string `json:"phone, omitempty"`
	}

	// AgreementDetails struct
	AgreementDetails struct {
		OutstandingBalance AmountPayout `json:"outstanding_balance"`
		CyclesRemaining    int          `json:"cycles_remaining,string"`
		CyclesCompleted    int          `json:"cycles_completed,string"`
		NextBillingDate    time.Time    `json:"next_billing_date"`
		LastPaymentDate    time.Time    `json:"last_payment_date"`
		LastPaymentAmount  AmountPayout `json:"last_payment_amount"`
		FinalPaymentDate   time.Time    `json:"final_payment_date"`
		FailedPaymentCount int          `json:"failed_payment_count,string"`
	}

	// Amount struct
	Amount struct {
		Currency string   `json:"currency"`
		Total    string   `json:"total"`
		Details  *Details `json:"details, omitempty"`
	}

	// AmountPayout struct
	AmountPayout struct {
		Currency string `json:"currency"`
		Value    string `json:"value"`
	}

	// ApplicationContext represents the application context, which customizes
	// the payer experience during the subscription approval process with PayPal.
	// ShippingPreference represents the location from which the shipping address is derived.
	// The possible values are:
	// ------------------------------------------------------------------------------------------------------------
	// | GET_FROM_FILE        | Get the customer-provided shipping address on the PayPal site.                    |
	// | NO_SHIPPING          | Redacts the shipping address from the PayPal site. Recommended for digital goods. |
	// | SET_PROVIDED_ADDRESS | Get the merchant-provided address. The customer cannot change this                |
	// |                      | address on the PayPal site. If merchant does not pass an address, customer        |
	// |                      | can choose the address on PayPal pages.                                           |
	// ------------------------------------------------------------------------------------------------------------
	// UserAction configures the label name to Continue or Subscribe Now for subscription consent experience.
	// The possible values are:
	// ------------------------------------------------------------------------------------------------------
	// | CONTINUE      | After you redirect the customer to the PayPal subscription consent page,			|
	// |               | a Continue button appears. Use this option when you want to control the activation |
	// |               | of the subscription and do not want PayPal to activate the subscription.			|
	// | SUBSCRIBE_NOW | After you redirect the customer to the PayPal subscription consent page, 			|
	// |               | a Subscribe Now button appears. Use this option when you want PayPal to activate   |
	// |               | the subscription.																	|
	// ------------------------------------------------------------------------------------------------------
	ApplicationContext struct {
		BrandName          string         `json:"brand_name, omitempty"`
		Locale             string         `json:"locale, omitempty"`
		LandingPage        string         `json:"landing_page, omitempty"`
		ShippingPreference string         `json:"shipping_preference, omitempty"` //default: GET_FROM_FILE
		UserAction         string         `json:"user_action, omitempty"`         //default: SUBSCRIBE_NOW
		PaymentMethod      *PaymentMethod `json:"payment_method, omitempty"`
		ReturnURL          string         `json:"return_url"`
		CancelURL          string         `json:"cancel_url"`
	}

	// Authorization struct
	Authorization struct {
		ID               string                `json:"id, omitempty"`
		CustomID         string                `json:"custom_id, omitempty"`
		InvoiceID        string                `json:"invoice_id, omitempty"`
		Status           string                `json:"status, omitempty"`
		StatusDetails    *CaptureStatusDetails `json:"status_details, omitempty"`
		Amount           *PurchaseUnitAmount   `json:"amount, omitempty"`
		SellerProtection *SellerProtection     `json:"seller_protection, omitempty"`
		CreateTime       *time.Time            `json:"create_time, omitempty"`
		UpdateTime       *time.Time            `json:"update_time, omitempty"`
		ExpirationTime   *time.Time            `json:"expiration_time, omitempty"`
		Links            []Link                `json:"links, omitempty"`
	}

	// AuthorizeOrderResponse .
	AuthorizeOrderResponse struct {
		CreateTime    *time.Time             `json:"create_time, omitempty"`
		UpdateTime    *time.Time             `json:"update_time, omitempty"`
		ID            string                 `json:"id, omitempty"`
		Status        string                 `json:"status, omitempty"`
		Intent        string                 `json:"intent, omitempty"`
		PurchaseUnits []PurchaseUnitRequest  `json:"purchase_units, omitempty"`
		Payer         *PayerWithNameAndPhone `json:"payer, omitempty"`
	}

	// AuthorizeOrderRequest - https://developer.paypal.com/docs/api/orders/v2/#orders_authorize
	AuthorizeOrderRequest struct {
		PaymentSource      *PaymentSource     `json:"payment_source, omitempty"`
		ApplicationContext ApplicationContext `json:"application_context, omitempty"`
	}

	// https://developer.paypal.com/docs/api/payments/v2/#definition-platform_fee
	PlatformFee struct {
		Amount *Money          `json:"amount, omitempty"`
		Payee  *PayeeForOrders `json:"payee, omitempty"`
	}

	// https://developer.paypal.com/docs/api/payments/v2/#definition-payment_instruction
	PaymentInstruction struct {
		PlatformFees     []PlatformFee `json:"platform_fees, omitempty"`
		DisbursementMode string        `json:"disbursement_mode, omitempty"`
	}

	// https://developer.paypal.com/docs/api/payments/v2/#authorizations_capture
	PaymentCaptureRequest struct {
		InvoiceID      string `json:"invoice_id, omitempty"`
		NoteToPayer    string `json:"note_to_payer, omitempty"`
		SoftDescriptor string `json:"soft_descriptor, omitempty"`
		Amount         *Money `json:"amount, omitempty"`
		FinalCapture   bool   `json:"final_capture, omitempty"`
	}

	SellerProtection struct {
		Status            string   `json:"status, omitempty"`
		DisputeCategories []string `json:"dispute_categories, omitempty"`
	}

	// https://developer.paypal.com/docs/api/payments/v2/#definition-capture_status_details
	CaptureStatusDetails struct {
		Reason string `json:"reason, omitempty"`
	}

	PaymentCaptureResponse struct {
		Status           string                `json:"status, omitempty"`
		StatusDetails    *CaptureStatusDetails `json:"status_details, omitempty"`
		ID               string                `json:"id, omitempty"`
		Amount           *Money                `json:"amount, omitempty"`
		InvoiceID        string                `json:"invoice_id, omitempty"`
		FinalCapture     bool                  `json:"final_capture, omitempty"`
		DisbursementMode string                `json:"disbursement_mode, omitempty"`
		Links            []Link                `json:"links, omitempty"`
	}

	// CaptureOrderRequest - https://developer.paypal.com/docs/api/orders/v2/#orders_capture
	CaptureOrderRequest struct {
		PaymentSource *PaymentSource `json:"payment_source"`
	}

	// BatchHeader struct
	BatchHeader struct {
		Amount            *AmountPayout      `json:"amount, omitempty"`
		Fees              *AmountPayout      `json:"fees, omitempty"`
		PayoutBatchID     string             `json:"payout_batch_id, omitempty"`
		BatchStatus       string             `json:"batch_status, omitempty"`
		TimeCreated       *time.Time         `json:"time_created, omitempty"`
		TimeCompleted     *time.Time         `json:"time_completed, omitempty"`
		SenderBatchHeader *SenderBatchHeader `json:"sender_batch_header, omitempty"`
	}

	// BillingAgreement struct
	BillingAgreement struct {
		Name                        string               `json:"name, omitempty"`
		Description                 string               `json:"description, omitempty"`
		StartDate                   JSONTime             `json:"start_date, omitempty"`
		Plan                        BillingPlan          `json:"plan, omitempty"`
		Payer                       Payer                `json:"payer, omitempty"`
		ShippingAddress             *ShippingAddress     `json:"shipping_address, omitempty"`
		OverrideMerchantPreferences *MerchantPreferences `json:"override_merchant_preferences, omitempty"`
	}

	// BillingPlan struct
	BillingPlan struct {
		ID                  string               `json:"id, omitempty"`
		Name                string               `json:"name, omitempty"`
		Description         string               `json:"description, omitempty"`
		Type                string               `json:"type, omitempty"`
		PaymentDefinitions  []PaymentDefinition  `json:"payment_definitions, omitempty"`
		MerchantPreferences *MerchantPreferences `json:"merchant_preferences, omitempty"`
	}

	// Capture struct
	Capture struct {
		Amount         *Amount    `json:"amount, omitempty"`
		IsFinalCapture bool       `json:"is_final_capture"`
		CreateTime     *time.Time `json:"create_time, omitempty"`
		UpdateTime     *time.Time `json:"update_time, omitempty"`
		State          string     `json:"state, omitempty"`
		ParentPayment  string     `json:"parent_payment, omitempty"`
		ID             string     `json:"id, omitempty"`
		Links          []Link     `json:"links, omitempty"`
	}

	// ChargeModel struct
	ChargeModel struct {
		Type   string       `json:"type, omitempty"`
		Amount AmountPayout `json:"amount, omitempty"`
	}

	// Client represents a Paypal REST API Client
	Client struct {
		sync.Mutex
		Client               *http.Client
		ClientID             string
		Secret               string
		APIBase              string
		Log                  io.Writer // If user set log file name all requests will be logged there
		Token                *TokenResponse
		tokenExpiresAt       time.Time
		returnRepresentation bool
	}

	// CreditCard struct
	CreditCard struct {
		ID                 string   `json:"id, omitempty"`
		PayerID            string   `json:"payer_id, omitempty"`
		ExternalCustomerID string   `json:"external_customer_id, omitempty"`
		Number             string   `json:"number"`
		Type               string   `json:"type"`
		ExpireMonth        string   `json:"expire_month"`
		ExpireYear         string   `json:"expire_year"`
		CVV2               string   `json:"cvv2, omitempty"`
		FirstName          string   `json:"first_name, omitempty"`
		LastName           string   `json:"last_name, omitempty"`
		BillingAddress     *Address `json:"billing_address, omitempty"`
		State              string   `json:"state, omitempty"`
		ValidUntil         string   `json:"valid_until, omitempty"`
	}

	// CreditCards GET /v1/vault/credit-cards
	CreditCards struct {
		Items      []CreditCard `json:"items"`
		Links      []Link       `json:"links"`
		TotalItems int          `json:"total_items"`
		TotalPages int          `json:"total_pages"`
	}

	// CreditCardToken struct
	CreditCardToken struct {
		CreditCardID string `json:"credit_card_id"`
		PayerID      string `json:"payer_id, omitempty"`
		Last4        string `json:"last4, omitempty"`
		ExpireYear   string `json:"expire_year, omitempty"`
		ExpireMonth  string `json:"expire_month, omitempty"`
	}

	// CreditCardsFilter struct
	CreditCardsFilter struct {
		PageSize int
		Page     int
	}

	// CreditCardField PATCH /v1/vault/credit-cards/credit_card_id
	CreditCardField struct {
		Operation string `json:"op"`
		Path      string `json:"path"`
		Value     string `json:"value"`
	}

	// Currency represents fee details (PayPal does not support all currencies.)
	Currency struct {
		Currency string `json:"currency"`
		Value    string `json:"value"`
	}

	// Details structure used in Amount structures as optional value
	Details struct {
		Subtotal         string `json:"subtotal, omitempty"`
		Shipping         string `json:"shipping, omitempty"`
		Tax              string `json:"tax, omitempty"`
		HandlingFee      string `json:"handling_fee, omitempty"`
		ShippingDiscount string `json:"shipping_discount, omitempty"`
		Insurance        string `json:"insurance, omitempty"`
		GiftWrap         string `json:"gift_wrap, omitempty"`
	}

	// ErrorResponseDetail struct
	ErrorResponseDetail struct {
		Field string `json:"field"`
		Issue string `json:"issue"`
		Links []Link `json:"link"`
	}

	// ErrorResponse https://developer.paypal.com/docs/api/errors/
	ErrorResponse struct {
		Response        *http.Response        `json:"-"`
		Name            string                `json:"name"`
		DebugID         string                `json:"debug_id"`
		Message         string                `json:"message"`
		InformationLink string                `json:"information_link"`
		Details         []ErrorResponseDetail `json:"details"`
	}

	// ExecuteAgreementResponse struct
	ExecuteAgreementResponse struct {
		ID               string           `json:"id"`
		State            string           `json:"state"`
		Description      string           `json:"description, omitempty"`
		Payer            Payer            `json:"payer"`
		Plan             BillingPlan      `json:"plan"`
		StartDate        time.Time        `json:"start_date"`
		ShippingAddress  ShippingAddress  `json:"shipping_address"`
		AgreementDetails AgreementDetails `json:"agreement_details"`
		Links            []Link           `json:"links"`
	}

	// ExecuteResponse struct
	ExecuteResponse struct {
		ID           string        `json:"id"`
		Links        []Link        `json:"links"`
		State        string        `json:"state"`
		Payer        PaymentPayer  `json:"payer"`
		Transactions []Transaction `json:"transactions, omitempty"`
	}

	// FundingInstrument struct
	FundingInstrument struct {
		CreditCard      *CreditCard      `json:"credit_card, omitempty"`
		CreditCardToken *CreditCardToken `json:"credit_card_token, omitempty"`
	}

	// Item struct
	Item struct {
		Name        string `json:"name"`
		UnitAmount  *Money `json:"unit_amount, omitempty"`
		Tax         *Money `json:"tax, omitempty"`
		Quantity    string `json:"quantity"`
		Description string `json:"description, omitempty"`
		SKU         string `json:"sku, omitempty"`
		Category    string `json:"category, omitempty"`
	}

	// ItemList struct
	ItemList struct {
		Items           []Item           `json:"items, omitempty"`
		ShippingAddress *ShippingAddress `json:"shipping_address, omitempty"`
	}

	// Link struct
	Link struct {
		Href        string `json:"href"`
		Rel         string `json:"rel, omitempty"`
		Method      string `json:"method, omitempty"`
		Description string `json:"description, omitempty"`
		Enctype     string `json:"enctype, omitempty"`
	}

	// PurchaseUnitAmount struct
	PurchaseUnitAmount struct {
		Currency  string                       `json:"currency_code"`
		Value     string                       `json:"value"`
		Breakdown *PurchaseUnitAmountBreakdown `json:"breakdown, omitempty"`
	}

	// PurchaseUnitAmountBreakdown struct
	PurchaseUnitAmountBreakdown struct {
		ItemTotal        *Money `json:"item_total, omitempty"`
		Shipping         *Money `json:"shipping, omitempty"`
		Handling         *Money `json:"handling, omitempty"`
		TaxTotal         *Money `json:"tax_total, omitempty"`
		Insurance        *Money `json:"insurance, omitempty"`
		ShippingDiscount *Money `json:"shipping_discount, omitempty"`
		Discount         *Money `json:"discount, omitempty"`
	}

	// Money represents the amount. For regular pricing, it is limited to a 20% increase
	// from the current amount and the change is applicable for both existing and future subscriptions. For trial period pricing,
	// there is no limit or constraint in changing the amount and the change is applicable only on future subscriptions.
	// The value, which might be:
	// An integer for currencies like JPY that are not typically fractional.
	// A decimal fraction for currencies like TND that are subdivided into thousandths.
	// For the required number of decimal places for a currency code, see https://developer.paypal.com/docs/api/reference/currency-codes/.
	// https://developer.paypal.com/docs/api/orders/v2/#definition-money
	Money struct {
		Currency string `json:"currency_code"`
		Value    string `json:"value"`
	}

	// PurchaseUnit struct
	PurchaseUnit struct {
		ReferenceID string              `json:"reference_id"`
		Amount      *PurchaseUnitAmount `json:"amount, omitempty"`
	}

	// TaxInfo used for orders.
	TaxInfo struct {
		TaxID     string `json:"tax_id, omitempty"`
		TaxIDType string `json:"tax_id_type, omitempty"`
	}

	// PhoneWithTypeNumber struct for PhoneWithType
	PhoneWithTypeNumber struct {
		NationalNumber string `json:"national_number, omitempty"`
	}

	// PhoneWithType struct used for orders
	PhoneWithType struct {
		PhoneType   string               `json:"phone_type, omitempty"`
		PhoneNumber *PhoneWithTypeNumber `json:"phone_number, omitempty"`
	}

	// CreateOrderPayerName create order payer name
	CreateOrderPayerName struct {
		GivenName string `json:"given_name, omitempty"`
		Surname   string `json:"surname, omitempty"`
	}

	// CreateOrderPayer used with create order requests
	CreateOrderPayer struct {
		Name         *CreateOrderPayerName          `json:"name, omitempty"`
		EmailAddress string                         `json:"email_address, omitempty"`
		PayerID      string                         `json:"payer_id, omitempty"`
		Phone        *PhoneWithType                 `json:"phone, omitempty"`
		BirthDate    string                         `json:"birth_date, omitempty"`
		TaxInfo      *TaxInfo                       `json:"tax_info, omitempty"`
		Address      *ShippingDetailAddressPortable `json:"address, omitempty"`
	}

	// PurchaseUnitRequest struct
	PurchaseUnitRequest struct {
		ReferenceID    string              `json:"reference_id, omitempty"`
		Amount         *PurchaseUnitAmount `json:"amount"`
		Payee          *PayeeForOrders     `json:"payee, omitempty"`
		Description    string              `json:"description, omitempty"`
		CustomID       string              `json:"custom_id, omitempty"`
		InvoiceID      string              `json:"invoice_id, omitempty"`
		SoftDescriptor string              `json:"soft_descriptor, omitempty"`
		Items          []Item              `json:"items, omitempty"`
		Shipping       *ShippingDetail     `json:"shipping, omitempty"`
	}

	// MerchantPreferences struct
	MerchantPreferences struct {
		SetupFee                *AmountPayout `json:"setup_fee, omitempty"`
		ReturnURL               string        `json:"return_url, omitempty"`
		CancelURL               string        `json:"cancel_url, omitempty"`
		AutoBillAmount          string        `json:"auto_bill_amount, omitempty"`
		InitialFailAmountAction string        `json:"initial_fail_amount_action, omitempty"`
		MaxFailAttempts         string        `json:"max_fail_attempts, omitempty"`
	}

	// Order struct
	Order struct {
		ID            string         `json:"id, omitempty"`
		Status        string         `json:"status, omitempty"`
		Intent        string         `json:"intent, omitempty"`
		PurchaseUnits []PurchaseUnit `json:"purchase_units, omitempty"`
		Links         []Link         `json:"links, omitempty"`
		CreateTime    *time.Time     `json:"create_time, omitempty"`
		UpdateTime    *time.Time     `json:"update_time, omitempty"`
	}

	// CaptureAmount struct
	CaptureAmount struct {
		ID       string              `json:"id, omitempty"`
		CustomID string              `json:"custom_id, omitempty"`
		Amount   *PurchaseUnitAmount `json:"amount, omitempty"`
	}

	// CapturedPayments has the amounts for a captured order
	CapturedPayments struct {
		Captures []CaptureAmount `json:"captures, omitempty"`
	}

	// CapturedPurchaseItem are items for a captured order
	CapturedPurchaseItem struct {
		Quantity    string `json:"quantity"`
		Name        string `json:"name"`
		SKU         string `json:"sku, omitempty"`
		Description string `json:"description, omitempty"`
	}

	// CapturedPurchaseUnit are purchase units for a captured order
	CapturedPurchaseUnit struct {
		Items    []CapturedPurchaseItem `json:"items, omitempty"`
		Payments *CapturedPayments      `json:"payments, omitempty"`
	}

	// PayerWithNameAndPhone struct
	PayerWithNameAndPhone struct {
		Name         *CreateOrderPayerName `json:"name, omitempty"`
		EmailAddress string                `json:"email_address, omitempty"`
		Phone        *PhoneWithType        `json:"phone, omitempty"`
		PayerID      string                `json:"payer_id, omitempty"`
	}

	// CaptureOrderResponse is the response for capture order
	CaptureOrderResponse struct {
		ID            string                 `json:"id, omitempty"`
		Status        string                 `json:"status, omitempty"`
		Payer         *PayerWithNameAndPhone `json:"payer, omitempty"`
		PurchaseUnits []CapturedPurchaseUnit `json:"purchase_units, omitempty"`
	}

	// Payer struct
	Payer struct {
		PaymentMethod      string              `json:"payment_method"`
		FundingInstruments []FundingInstrument `json:"funding_instruments, omitempty"`
		PayerInfo          *PayerInfo          `json:"payer_info, omitempty"`
		Status             string              `json:"payer_status, omitempty"`
	}

	// PayerInfo struct
	PayerInfo struct {
		Email           string           `json:"email, omitempty"`
		FirstName       string           `json:"first_name, omitempty"`
		LastName        string           `json:"last_name, omitempty"`
		PayerID         string           `json:"payer_id, omitempty"`
		Phone           string           `json:"phone, omitempty"`
		ShippingAddress *ShippingAddress `json:"shipping_address, omitempty"`
		TaxIDType       string           `json:"tax_id_type, omitempty"`
		TaxID           string           `json:"tax_id, omitempty"`
		CountryCode     string           `json:"country_code"`
	}

	// PaymentDefinition struct
	PaymentDefinition struct {
		ID                string        `json:"id, omitempty"`
		Name              string        `json:"name, omitempty"`
		Type              string        `json:"type, omitempty"`
		Frequency         string        `json:"frequency, omitempty"`
		FrequencyInterval string        `json:"frequency_interval, omitempty"`
		Amount            AmountPayout  `json:"amount, omitempty"`
		Cycles            string        `json:"cycles, omitempty"`
		ChargeModels      []ChargeModel `json:"charge_models, omitempty"`
	}

	// PaymentOptions struct
	PaymentOptions struct {
		AllowedPaymentMethod string `json:"allowed_payment_method, omitempty"`
	}

	// PaymentPatch PATCH /v2/payments/payment/{payment_id)
	PaymentPatch struct {
		Operation string      `json:"op"`
		Path      string      `json:"path"`
		Value     interface{} `json:"value"`
	}

	// PaymentPayer struct
	PaymentPayer struct {
		PaymentMethod string     `json:"payment_method"`
		Status        string     `json:"status, omitempty"`
		PayerInfo     *PayerInfo `json:"payer_info, omitempty"`
	}

	// PaymentResponse structure
	PaymentResponse struct {
		ID           string        `json:"id"`
		State        string        `json:"state"`
		Intent       string        `json:"intent"`
		Payer        Payer         `json:"payer"`
		Transactions []Transaction `json:"transactions"`
		Links        []Link        `json:"links"`
	}

	// PaymentSource represents the payment source definitions
	PaymentSource struct {
		Card  *PaymentSourceCard  `json:"card"`
		Token *PaymentSourceToken `json:"token"`
	}

	// PaymentSourceCard represents card details
	// SecurityCode represents the three- or four-digit security code of the card. Also known as the CVV, CVC, CVN, CVE, or CID.
	PaymentSourceCard struct {
		ID             string           `json:"id, omitempty"`
		Name           string           `json:"name, omitempty"`
		Number         string           `json:"number"`
		Expiry         string           `json:"expiry"`
		SecurityCode   string           `json:"security_code, omitempty"`
		LastDigits     string           `json:"last_digits, omitempty"`
		CardType       string           `json:"card_type, omitempty"`
		BillingAddress *AddressPortable `json:"billing_address, omitempty"`
	}

	// AddressPortable represents address details
	// More info -> https://developer.paypal.com/docs/api/subscriptions/v1/#definition-address_portable
	AddressPortable struct {
		AddressLine1 string `json:"address_line_1"`
		AddressLine2 string `json:"address_line_2"`
		AddressLine3 string `json:"address_line_3"`
		AdminArea4   string `json:"admin_area_4"`
		AdminArea3   string `json:"admin_area_3"`
		AdminArea2   string `json:"admin_area_2"`
		AdminArea1   string `json:"admin_area_1"`
		PostalCode   string `json:"postal_code"`
		CountryCode  string `json:"country_code"`
	}

	// PaymentSourceToken structure
	PaymentSourceToken struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}

	// Payout struct
	Payout struct {
		SenderBatchHeader *SenderBatchHeader `json:"sender_batch_header"`
		Items             []PayoutItem       `json:"items"`
	}

	// PayoutItem struct
	PayoutItem struct {
		RecipientType string        `json:"recipient_type"`
		Receiver      string        `json:"receiver"`
		Amount        *AmountPayout `json:"amount"`
		Note          string        `json:"note, omitempty"`
		SenderItemID  string        `json:"sender_item_id, omitempty"`
	}

	// PayoutItemResponse struct
	PayoutItemResponse struct {
		PayoutItemID      string        `json:"payout_item_id"`
		TransactionID     string        `json:"transaction_id"`
		TransactionStatus string        `json:"transaction_status"`
		PayoutBatchID     string        `json:"payout_batch_id, omitempty"`
		PayoutItemFee     *AmountPayout `json:"payout_item_fee, omitempty"`
		PayoutItem        *PayoutItem   `json:"payout_item"`
		TimeProcessed     *time.Time    `json:"time_processed, omitempty"`
		Links             []Link        `json:"links"`
		Error             ErrorResponse `json:"errors, omitempty"`
	}

	// PayoutResponse struct
	PayoutResponse struct {
		BatchHeader *BatchHeader         `json:"batch_header"`
		Items       []PayoutItemResponse `json:"items"`
		Links       []Link               `json:"links"`
	}

	// RedirectURLs struct
	RedirectURLs struct {
		ReturnURL string `json:"return_url, omitempty"`
		CancelURL string `json:"cancel_url, omitempty"`
	}

	// Refund struct
	Refund struct {
		ID            string     `json:"id, omitempty"`
		Amount        *Amount    `json:"amount, omitempty"`
		CreateTime    *time.Time `json:"create_time, omitempty"`
		State         string     `json:"state, omitempty"`
		CaptureID     string     `json:"capture_id, omitempty"`
		ParentPayment string     `json:"parent_payment, omitempty"`
		UpdateTime    *time.Time `json:"update_time, omitempty"`
	}

	// RefundResponse .
	RefundResponse struct {
		ID     string              `json:"id, omitempty"`
		Amount *PurchaseUnitAmount `json:"amount, omitempty"`
		Status string              `json:"status, omitempty"`
	}

	// Related struct
	Related struct {
		Sale          *Sale          `json:"sale, omitempty"`
		Authorization *Authorization `json:"authorization, omitempty"`
		Order         *Order         `json:"order, omitempty"`
		Capture       *Capture       `json:"capture, omitempty"`
		Refund        *Refund        `json:"refund, omitempty"`
	}

	// Sale represents the details for a sale
	// PaymentMode represents the transaction payment mode. Supported only for PayPal payments.
	// Possible values: INSTANT_TRANSFER, MANUAL_BANK_TRANSFER, DELAYED_TRANSFER, ECHECK.
	// State represents the state of the sale transaction. Possible values: completed, partially_refunded, pending, refunded, denied.
	// ReasonCode represents a code that describes why the transaction state is pending or reversed. Supported only for PayPal payments.
	// Possible values: CHARGEBACK, GUARANTEE, BUYER_COMPLAINT, REFUND, UNCONFIRMED_SHIPPING_ADDRESS, ECHECK, INTERNATIONAL_WITHDRAWAL,
	// RECEIVING_PREFERENCE_MANDATES_MANUAL_ACTION, PAYMENT_REVIEW, REGULATORY_REVIEW, UNILATERAL, VERIFICATION_REQUIRED, TRANSACTION_APPROVED_AWAITING_FUNDING.
	// ProtectionEligibility represents the merchant protection level in effect for the transaction. Supported only for PayPal payments.
	// Possible values: ELIGIBLE, PARTIALLY_ELIGIBLE, INELIGIBLE.
	// ProtectionEligibilityType represents the merchant protection type in effect for the transaction. Returned only when protection_eligibility is ELIGIBLE or
	// PARTIALLY_ELIGIBLE. Supported only for PayPal payments. Possible values: ITEM_NOT_RECEIVED_ELIGIBLE, UNAUTHORIZED_PAYMENT_ELIGIBLE,
	// ITEM_NOT_RECEIVED_ELIGIBLE,UNAUTHORIZED_PAYMENT_ELIGIBLE.
	// PaymentHoldStatus represents the recipient fund status. Returned only when the fund status is held. Possible values: HELD.
	// ProcessorResponse represents the processor-provided response codes that describe the submitted payment. Supported only when the payment_method is credit_card.
	Sale struct {
		ID                        string               `json:"id, omitempty"`
		Amount                    *Amount              `json:"amount, omitempty"`
		PaymentMode               string               `json:"payment_mode, omitempty"`                //Read only
		State                     string               `json:"state, omitempty"`                       //Read only
		ReasonCode                string               `json:"reason_code, omitempty"`                 //Read only
		ProtectionEligibility     string               `json:"protection_eligibility, omitempty"`      //Read only
		ProtectionEligibilityType string               `json:"protection_eligibility_type, omitempty"` //Read only
		ClearingTime              string               `json:"clearing_time, omitempty"`               //Read only
		PaymentHoldStatus         string               `json:"payment_hold_status, omitempty"`         //Read only
		PaymentHoldReasons        []*PaymentHoldReason `json:"payment_hold_reasons, omitempty"`        //Read only
		TransactionFee            *Currency            `json:"transaction_fee, omitempty"`             //Read only
		ReceivableAmount          *Currency            `json:"receivable_amount, omitempty"`
		ExchangeRage              string               `json:"exchange_rage, omitempty"` //Read only
		FmfDetails                *FmfDetails          `json:"fmf_details, omitempty"`
		ReceiptID                 string               `json:"receipt_id, omitempty"`     //Read only
		ParentPayment             string               `json:"parent_payment, omitempty"` //Read only
		ProcessorResponse         *ProcessorResponse   `json:"processor_response, omitempty"`
		InvoiceNumber             string               `json:"invoice_number, omitempty"`       //Read only
		BillingAgreementID        string               `json:"billing_agreement_id, omitempty"` //Read only
		CreateTime                string               `json:"create_time, omitempty"`          //Read only
		UpdateTime                string               `json:"update_time, omitempty"`          //Read only
		Links                     []*Link              `json:"links, omitempty"`                //Read only
	}

	// PaymentHoldReason represents the reason that PayPal holds the recipient fund.  Set only if the payment hold status is HELD.
	// Possible values: PAYMENT_HOLD, SHIPPING_RISK_HOLD.
	PaymentHoldReason struct {
		PaymentHoldReason string `json:"payment_hold_reason, omitempty"`
	}

	// FmfDetails represents the Fraud Management Filter (FMF) details that are applied to the payment that result in an accept, deny,
	// or pending action. Returned in a payment response only if the merchant has enabled FMF in the profile settings and one of the
	// fraud filters was triggered based on those settings. For more information, see Fraud Management Filters Summary.
	// FilterType represents the filter type. The possible values are:
	// --------------------------------------
	// | ACCEPT  | The accept filter type.  |
	// | PENDING | The pending filter type. |
	// | DENY    | The deny filter type.    |
	// | REPORT  | The report filter type.  |
	// --------------------------------------
	// FilterID represents the filter ID. The possible values are:
	// ------------------------------------------------------------------------------------
	// | AVS_NO_MATCH                       	| AVS no match. 						  |
	// | AVS_PARTIAL_MATCH 						| AVS partial match.                      |
	// | AVS_UNAVAILABLE_OR_UNSUPPORTED 		| AVS unavailable or unsupported. 	  	  |
	// | CARD_SECURITY_CODE_MISMATCH 			| Card security code mismatch. 			  |
	// | MAXIMUM_TRANSACTION_AMOUNT 			| The maximum transaction amount. 		  |
	// | UNCONFIRMED_ADDRESS 					| Unconfirmed address. 					  |
	// | COUNTRY_MONITOR 						| Country monitor. 						  |
	// | LARGE_ORDER_NUMBER 					| Large order number. 					  |
	// | BILLING_OR_SHIPPING_ADDRESS_MISMATCH   | Billing or shipping address mismatch.   |
	// | RISKY_ZIP_CODE    					    | Risky zip code. 						  |
	// | SUSPECTED_FREIGHT_FORWARDER_CHECK 	    | Suspected freight forwarder check. 	  |
	// | TOTAL_PURCHASE_PRICE_MINIMUM           | Total purchase price minimum. 		  |
	// | IP_ADDRESS_VELOCITY                    | IP address velocity. 					  |
	// | RISKY_EMAIL_ADDRESS_DOMAIN_CHECK	    | Risky email address domain check. 	  |
	// | RISKY_BANK_IDENTIFICATION_NUMBER_CHECK | Risky bank identification number check. |
	// | RISKY_IP_ADDRESS_RANGE                 | Risky IP address range. 				  |
	// | PAYPAL_FRAUD_MODEL 					| PayPal fraud model. 					  |
	// ------------------------------------------------------------------------------------
	FmfDetails struct {
		FilterType  string `json:"filter_type"`            //Read only
		FilterID    string `json:"filter_id"`              //Read only
		Name        string `json:"name, omitempty"`        //Read only
		Description string `json:"description, omitempty"` //Read only

	}

	// ProcessorResponse represents the processor-provided response codes that describe the submitted payment.
	// Supported only when the payment_method is credit_card.
	// AdviceCode represents the merchant advice on how to handle declines for recurring payments. The possible values are:
	// ------------------------------------------------------------------------------------------------------------------------------
	// | 01_NEW_ACCOUNT_INFORMATION                                  | 01 New account information. 									|
	// | 02_TRY_AGAIN_LATER 										 | 02 Try again later. 											|
	// | 02_STOP_SPECIFIC_PAYMENT  									 | 02 Stop specific payment. 									|
	// | 03_DO_NOT_TRY_AGAIN 										 | 03 Do not try again. 										|
	// | 03_REVOKE_AUTHORIZATION_FOR_FUTURE_PAYMENT 				 | 03 Revoke authorization for future payment. 					|
	// | 21_DO_NOT_TRY_AGAIN_CARD_HOLDER_CANCELLED_RECURRRING_CHARGE | 21 Do not try again. Card holder cancelled recurring charge. |
	// | 21_CANCEL_ALL_RECURRING_PAYMENTS 							 | 21 Cancel all recurring payments. 							|
	// ------------------------------------------------------------------------------------------------------------------------------
	ProcessorResponse struct {
		ResponseCode string `json:"response_code"`            //Read only
		AvsCode      string `json:"avs_code, omitempty"`      //Read only
		CvvCode      string `json:"cvv_code, omitempty"`      //Read only
		AdviceCode   string `json:"advice_code, omitempty"`   //Read only
		EciSubmitted string `json:"eci_submitted, omitempty"` //Read only
		Vpas         string `json:"vpas, omitempty"`          //Read only
	}

	// SenderBatchHeader struct
	SenderBatchHeader struct {
		EmailSubject  string `json:"email_subject"`
		SenderBatchID string `json:"sender_batch_id, omitempty"`
	}

	// ShippingAddress struct
	ShippingAddress struct {
		RecipientName string `json:"recipient_name, omitempty"`
		Type          string `json:"type, omitempty"`
		Line1         string `json:"line1"`
		Line2         string `json:"line2, omitempty"`
		City          string `json:"city"`
		CountryCode   string `json:"country_code"`
		PostalCode    string `json:"postal_code, omitempty"`
		State         string `json:"state, omitempty"`
		Phone         string `json:"phone, omitempty"`
	}

	// ShippingDetailAddressPortable represents address details
	// More info -> https://developer.paypal.com/docs/api/subscriptions/v1/#definition-shipping_detail.address_portable
	ShippingDetailAddressPortable struct {
		AddressLine1 string `json:"address_line_1, omitempty"`
		AddressLine2 string `json:"address_line_2, omitempty"`
		AdminArea1   string `json:"admin_area_1, omitempty"`
		AdminArea2   string `json:"admin_area_2, omitempty"`
		PostalCode   string `json:"postal_code, omitempty"`
		CountryCode  string `json:"country_code, omitempty"`
	}

	// ShippingDetailsName represents shipping details full name
	ShippingDetailsName struct {
		FullName string `json:"full_name, omitempty"`
	}

	// ShippingDetail represents the shipping details
	ShippingDetail struct {
		Name    *ShippingDetailsName           `json:"name, omitempty"`
		Address *ShippingDetailAddressPortable `json:"address, omitempty"`
	}

	expirationTime int64

	// TokenResponse is for API response for the /oauth2/token endpoint
	TokenResponse struct {
		RefreshToken string         `json:"refresh_token"`
		Token        string         `json:"access_token"`
		Type         string         `json:"token_type"`
		ExpiresIn    expirationTime `json:"expires_in"`
	}

	// Since it is not used i change it @gligor
	// Transaction represents details about transaction
	Transaction struct {
		ID                  string               `json:"id, omitempty"`                    //Read only
		Status              string               `json:"status, omitempty"`                //Read only
		AmountWithBreakdown *AmountWithBreakdown `json:"amount_with_breakdown, omitempty"` //Read only
		PayerName           *Name                `json:"payer_name, omitempty"`            //Read only
		PayerEmail          string               `json:"payer_email, omitempty"`           //Read only
		Time                string               `json:"time, omitempty"`                  //Read only
	}

	// AmountWithBreakdown represents the breakdown details for the amount. Includes the gross, tax, fee, and shipping amounts.
	AmountWithBreakdown struct {
		GrossAmount    *Money `json:"gross_amount"`               //Read only
		FeeAmount      *Money `json:"fee_amount, omitempty"`      //Read only
		ShippingAmount *Money `json:"shipping_amount, omitempty"` //Read only
		TaxAmount      *Money `json:"tax_amount, omitempty"`      //Read only
		NetAmount      *Money `json:"net_amount"`                 //Read only
	}

	//Payee struct
	Payee struct {
		Email string `json:"email"`
	}

	// PayeeForOrders struct
	PayeeForOrders struct {
		EmailAddress string `json:"email_address, omitempty"`
		MerchantID   string `json:"merchant_id, omitempty"`
	}

	// UserInfo struct
	UserInfo struct {
		ID              string   `json:"user_id"`
		Name            string   `json:"name"`
		GivenName       string   `json:"given_name"`
		FamilyName      string   `json:"family_name"`
		Email           string   `json:"email"`
		Verified        bool     `json:"verified, omitempty,string"`
		Gender          string   `json:"gender, omitempty"`
		BirthDate       string   `json:"birthdate, omitempty"`
		ZoneInfo        string   `json:"zoneinfo, omitempty"`
		Locale          string   `json:"locale, omitempty"`
		Phone           string   `json:"phone_number, omitempty"`
		Address         *Address `json:"address, omitempty"`
		VerifiedAccount bool     `json:"verified_account, omitempty,string"`
		AccountType     string   `json:"account_type, omitempty"`
		AgeRange        string   `json:"age_range, omitempty"`
		PayerID         string   `json:"payer_id, omitempty"`
	}

	// WebProfile represents the configuration of the payment web payment experience
	//
	// https://developer.paypal.com/docs/api/payment-experience/
	WebProfile struct {
		ID           string       `json:"id, omitempty"`
		Name         string       `json:"name"`
		Presentation Presentation `json:"presentation, omitempty"`
		InputFields  InputFields  `json:"input_fields, omitempty"`
		FlowConfig   FlowConfig   `json:"flow_config, omitempty"`
	}

	// Presentation represents the branding and locale that a customer sees on
	// redirect payments
	//
	// https://developer.paypal.com/docs/api/payment-experience/#definition-presentation
	Presentation struct {
		BrandName  string `json:"brand_name, omitempty"`
		LogoImage  string `json:"logo_image, omitempty"`
		LocaleCode string `json:"locale_code, omitempty"`
	}

	// InputFields represents the fields that are displayed to a customer on
	// redirect payments
	//
	// https://developer.paypal.com/docs/api/payment-experience/#definition-input_fields
	InputFields struct {
		AllowNote       bool `json:"allow_note, omitempty"`
		NoShipping      uint `json:"no_shipping, omitempty"`
		AddressOverride uint `json:"address_override, omitempty"`
	}

	// FlowConfig represents the general behaviour of redirect payment pages
	//
	// https://developer.paypal.com/docs/api/payment-experience/#definition-flow_config
	FlowConfig struct {
		LandingPageType   string `json:"landing_page_type, omitempty"`
		BankTXNPendingURL string `json:"bank_txn_pending_url, omitempty"`
		UserAction        string `json:"user_action, omitempty"`
	}

	VerifyWebhookResponse struct {
		VerificationStatus string `json:"verification_status, omitempty"`
	}

	WebhookEvent struct {
		ID              string    `json:"id"`
		CreateTime      time.Time `json:"create_time"`
		ResourceType    string    `json:"resource_type"`
		EventType       string    `json:"event_type"`
		Summary         string    `json:"summary, omitempty"`
		Resource        Resource  `json:"resource"`
		Links           []Link    `json:"links"`
		EventVersion    string    `json:"event_version, omitempty"`
		ResourceVersion string    `json:"resource_version, omitempty"`
	}

	Resource struct {
		// Payment Resource type
		ID                     string                  `json:"id, omitempty"`
		Status                 string                  `json:"status, omitempty"`
		StatusDetails          *CaptureStatusDetails   `json:"status_details, omitempty"`
		Amount                 *PurchaseUnitAmount     `json:"amount, omitempty"`
		UpdateTime             string                  `json:"update_time, omitempty"`
		CreateTime             string                  `json:"create_time, omitempty"`
		ExpirationTime         string                  `json:"expiration_time, omitempty"`
		SellerProtection       *SellerProtection       `json:"seller_protection, omitempty"`
		FinalCapture           bool                    `json:"final_capture, omitempty"`
		SellerPayableBreakdown *CaptureSellerBreakdown `json:"seller_payable_breakdown, omitempty"`
		NoteToPayer            string                  `json:"note_to_payer, omitempty"`
		// merchant-onboarding Resource type
		PartnerClientID string `json:"partner_client_id, omitempty"`
		MerchantID      string `json:"merchant_id, omitempty"`
		// Common
		Links []Link `json:"links, omitempty"`
	}

	CaptureSellerBreakdown struct {
		GrossAmount         PurchaseUnitAmount  `json:"gross_amount"`
		PayPalFee           PurchaseUnitAmount  `json:"paypal_fee"`
		NetAmount           PurchaseUnitAmount  `json:"net_amount"`
		TotalRefundedAmount *PurchaseUnitAmount `json:"total_refunded_amount, omitempty"`
	}

	ReferralRequest struct {
		TrackingID            string                 `json:"tracking_id"`
		PartnerConfigOverride *PartnerConfigOverride `json:"partner_config_override,omitemtpy"`
		Operations            []Operation            `json:"operations, omitempty"`
		Products              []string               `json:"products, omitempty"`
		LegalConsents         []Consent              `json:"legal_consents, omitempty"`
	}

	PartnerConfigOverride struct {
		PartnerLogoURL       string `json:"partner_logo_url, omitempty"`
		ReturnURL            string `json:"return_url, omitempty"`
		ReturnURLDescription string `json:"return_url_description, omitempty"`
		ActionRenewalURL     string `json:"action_renewal_url, omitempty"`
		ShowAddCreditCard    *bool  `json:"show_add_credit_card, omitempty"`
	}

	Operation struct {
		Operation                string              `json:"operation"`
		APIIntegrationPreference *IntegrationDetails `json:"api_integration_preference, omitempty"`
	}

	IntegrationDetails struct {
		RestAPIIntegration *RestAPIIntegration `json:"rest_api_integration, omitempty"`
	}

	RestAPIIntegration struct {
		IntegrationMethod string            `json:"integration_method"`
		IntegrationType   string            `json:"integration_type"`
		ThirdPartyDetails ThirdPartyDetails `json:"third_party_details"`
	}

	ThirdPartyDetails struct {
		Features []string `json:"features"`
	}

	Consent struct {
		Type    string `json:"type"`
		Granted bool   `json:"granted"`
	}

	DefaultResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	// CreateProductRequest represents body parameters needed to create PayPal product
	// Type represents the product type. Indicates whether the product is physical or tangible goods, or a service. The allowed values are:
	// ---------------------------------------------------------
	// | PHYSICAL | Physical goods.							   |
	// | PHYSICAL | Digital goods.							   |
	// | PHYSICAL | A service. For example, technical support. |
	// ---------------------------------------------------------
	// You can see category allowed values in PayPal docs -> https://developer.paypal.com/docs/api/catalog-products/v1/#products-create-request-body
	CreateProductRequest struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"` //default: PHYSICAL
		Category    string `json:"category, omitempty"`
		ImageUrl    string `json:"image_url, omitempty"`
		HomeUrl     string `json:"home_url, omitempty"`
	}

	// Product represents PayPal product
	// Type represents the product type. Indicates whether the product is physical or tangible goods, or a service. The allowed values are:
	// ---------------------------------------------------------
	// | PHYSICAL | Physical goods.							   |
	// | PHYSICAL | Digital goods.							   |
	// | PHYSICAL | A service. For example, technical support. |
	// ---------------------------------------------------------
	// You can see category allowed values in PayPal docs -> https://developer.paypal.com/docs/api/catalog-products/v1/#products-create-request-body
	Product struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Type        string  `json:"type"` //default: PHYSICAL
		Category    string  `json:"category, omitempty"`
		ImageUrl    string  `json:"image_url, omitempty"`
		HomeUrl     string  `json:"home_url, omitempty"`
		CreateTime  string  `json:"creat_time, omitempty"` //Read only
		UpdateTime  string  `json:"updat_time, omitempty"` //Read only
		Links       []*Link `json:"links, omitempty"`      //Read only
	}

	// ListProductsRequest represents query params for list products call
	ListProductsRequest struct {
		PageSize      uint64 `json:"page_size"`      //default: 10 min:1 max:20
		Page          uint64 `json:"page"`           //default: 1 min:1 max:100000
		TotalRequired bool   `json:"total_required"` //default: false
	}

	// ListProductsResponse represents the response od list products
	ListProductsResponse struct {
		TotalItems uint64     `json:"total_items, omitempty"` //min: 0, max: 500000000
		TotalPages uint64     `json:"total_pages, omitempty"` //min: 0, max: 100000000
		Products   []*Product `json:"products"`
		Links      []*Link    `json:"links"` //Read only
	}

	// PatchObject represents the object used for updating PayPal objects
	PatchObject struct {
		Operation string `json:"op"`
		Path      string `json:"path"`
		Value     string `json:"value"`
	}

	//Resource v1 for old hooks
	SuspendBillingAgreementV1Response struct {
		ID       string             `json:"id"`
		Resource BillingAgreementV1 `json:"resource"`
	}

	BillingAgreementV1 struct {
		ID               string `json:"id"`
		PlanID           string `json:"plan_id"`
		Status           string `json:"status"`
		StatusUpdateTime string `json:"status_update_time"`
	}

	// CreatePlan represents body parameters needed to create PayPal plan
	// Status represents the initial state of the plan. Allowed input values are CREATED and ACTIVE. The allowed values are:
	// ----------------------------------------------------------------------------------------------
	// | CREATED  | The plan was created. You cannot create subscriptions for a plan in this state. |
	// | INACTIVE | The plan is inactive.															|
	// | ACTIVE   | The plan is active. You can only create subscriptions for a plan in this state. |
	// ----------------------------------------------------------------------------------------------
	CreatePlan struct {
		ProductID          string              `json:"product_id"`
		Name               string              `json:"name"`
		Status             string              `json:"status"` //default: ACTIVE
		Description        string              `json:"description, omitempty"`
		BillingCycles      []*BillingCycle     `json:"billing_cycles"`
		PaymentPreferences *PaymentPreferences `json:"payment_preferences"`
		Taxes              *Taxes              `json:"taxes, omitempty"`
		QuantitySupported  bool                `json:"quantity_supported, omitempty"`
	}

	// BillingCycle represents the cycles for billing the subscription
	// The tenure type of the billing cycle. In case of a plan having trial period, only 1 trial period is allowed per plan. The possible values are:
	// --------------------------------------
	// | REGULAR | A regular billing cycle. |
	// | TRIAL   | A trial billing cycle.   |
	// --------------------------------------
	BillingCycle struct {
		PricingScheme *PricingScheme `json:"pricing_scheme, omitempty"` //Free Trial Cycle doesn't require scheme
		Frequency     *Frequency     `json:"frequency"`
		TenureType    string         `json:"tenure_type"`
		Sequence      uint64         `json:"sequence"`                //min: 0, max: 99
		TotalCycles   uint64         `json:"total_cycles, omitempty"` //default: 1, min: 0, max: 999
	}

	// PricingScheme represents the active pricing scheme for this billing cycle.
	// A free trial billing cycle does not require a pricing scheme.
	PricingScheme struct {
		Version    uint64 `json:"version, omitempty"` //Read only
		FixedPrice *Money `json:"fixed_price"`
		CreateTime string `json:"create_time, omitempty"` //Read only
		UpdateTime string `json:"update_time, omitempty"` //Read only
	}

	// Frequency represents the frequency details for this billing cycle.
	// Interval unit is the interval at which the subscription is charged or billed
	// These are the possible combinations
	// --------------------------------------
	// | Interval unit | Max Interval count |
	// --------------------------------------
	// | DAY           | 365                |
	// | WEEK          | 52                 |
	// | MONTH         | 12                 |
	// | YEAR          | 1                  |
	// --------------------------------------
	Frequency struct {
		IntervalUnit  string `json:"interval_unit"`
		IntervalCount uint64 `json:"interval_count, omitempty"`
	}

	// PaymentPreferences represents the payment preferences for a subscription
	// SetupFeeFailureAction represents the action to take on the subscription if the initial payment for the setup fails. The possible values are:
	// -------------------------------------------------------------------------------------
	// | CONTINUE | Continues the subscription if the initial payment for the setup fails. |
	// | CANCEL   | Cancels the subscription if the initial payment for the setup fails.   |
	// -------------------------------------------------------------------------------------
	PaymentPreferences struct {
		AutoBillOutstanding     bool   `json:"auto_bill_outstanding, omitempty"` //default true
		SetupFee                *Money `json:"setup_fee, omitempty"`
		ServiceType             string `json:"service_type, omitempty"`              //Read only
		SetupFeeFailureAction   string `json:"setup_fee_failure_action, omitempty"`  //default: CANCEL
		PaymentFailureThreshold uint64 `json:"payment_failure_threshold, omitempty"` //default: 0, min: 0, max: 999
	}

	// Taxes represents the tax details
	Taxes struct {
		Percentage string `json:"percentage"`
		Inclusive  bool   `json:"inclusive, omitempty"` //default: true
	}

	// Plan represents the details for the subscription plan for paying
	// Status represents the initial state of the plan. Allowed input values are CREATED and ACTIVE. The allowed values are:
	// ----------------------------------------------------------------------------------------------
	// | CREATED  | The plan was created. You cannot create subscriptions for a plan in this state. |
	// | INACTIVE | The plan is inactive.															|
	// | ACTIVE   | The plan is active. You can only create subscriptions for a plan in this state. |
	// ----------------------------------------------------------------------------------------------
	Plan struct {
		ID                 string              `json:"id"`
		ProductID          string              `json:"product_id"`
		Name               string              `json:"name"`
		Status             string              `json:"status"`
		Description        string              `json:"description, omitempty"`
		BillingCycles      []*BillingCycle     `json:"billing_cycles"`
		PaymentPreferences *PaymentPreferences `json:"payment_preferences"`
		Taxes              *Taxes              `json:"taxes, omitempty"`
		QuantitySupported  bool                `json:"quantity_supported, omitempty"`
		CreateTime         string              `json:"create_time"`           //Read only
		UpdateTime         string              `json:"update_time"`           //Read only
		Version            uint64              `json:"version, omitempty"`    //Read only
		UsageType          string              `json:"usage_type, omitempty"` //Read only
		Links              []*Link             `json:"links"`                 //Read only
	}

	// ListPlansParams represents query params for list products call
	ListPlansParams struct {
		ProductID     string `json:"product_id, omitempty"`
		PageSize      uint64 `json:"page_size, omitempty"`      //default: 10, min:1, max:20
		Page          uint64 `json:"page, omitempty"`           //default: 1, min:1, max:100000
		TotalRequired bool   `json:"total_required, omitempty"` //default: false
	}

	// ListPlansResponse represents the list of the plans
	ListPlansResponse struct {
		TotalItems uint64  `json:"total_items, omitempty"` //min: 0, max: 500000000
		TotalPages uint64  `json:"total_pages, omitempty"` //min: 0, max: 100000000
		Products   []*Plan `json:"plans"`
		Links      []*Link `json:"links"` //Read only
	}

	// UpdatePricingSchemasListRequest represents an array of pricing schemes to update plans billing cycles
	UpdatePricingSchemasListRequest struct {
		PricingSchemes []*UpdatePricingSchemaRequest `json:"pricing_schemes"`
	}

	// UpdatePricingSchemaRequest represents a pricing schema to update plans billing cycle
	UpdatePricingSchemaRequest struct {
		BillingCycleSequence uint64         `json:"billing_cycle_sequence"` //min: 1, max: 99
		PricingScheme        *PricingScheme `json:"pricing_scheme"`
	}

	// PaymentMethod represents the customer and merchant payment preferences
	// Currently only PAYPAl payment is supported
	// PayeePreferred represents the merchant-preferred payment sources. The possible values are:
	// -------------------------------------------------------------------------------------------------
	// | UNRESTRICTED 				| Accepts any type of payment from the customer.				   |
	// | IMMEDIATE_PAYMENT_REQUIRED | Accepts only immediate payment from the customer. For example,   |
	// | 							| credit card, PayPal balance, or instant ACH. Ensures that at the |
	// | 							| time of capture, the payment does not have the `pending` status. |
	// -------------------------------------------------------------------------------------------------
	// Category provides context (e.g. frequency of payment (Single, Recurring) along with whether
	// (Customer is Present, Not Present) for the payment being processed. For Card and PayPal Vaulted/Billing
	// Agreement transactions, this helps specify the appropriate indicators to the networks
	// (e.g. Mastercard, Visa) which ensures compliance as well as ensure a better auth-rate.
	// For bank processing, indicates to clearing house whether the transaction is recurring or not depending
	// on the option chosen. The possible values are:
	// --------------------------------------------------------------------------------------------------------------
	// | CUSTOMER_PRESENT_SINGLE_PURCHASE | If the payments is an e-commerce payment initiated by the customer.     |
	// | 								  | Customer typically enters payment information (e.g. card number,  	 	|
	// | 								  | approves payment within the PayPal Checkout flow) and such information  |
	// | 								  | that has not been previously stored on file.							|
	// | CUSTOMER_NOT_PRESENT_RECURRING   | Subsequent recurring payments (e.g. subscriptions with a fixed amount	|
	// |				 				  | on a predefined schedule when customer is not present.					|
	// | CUSTOMER_PRESENT_RECURRING_FIRST | The first payment initiated by the customer which is expected to be		|
	// | 								  | followed by a series of subsequent recurring payments transactions		|
	// | 								  | (e.g. subscriptions with a fixed amount on a predefined schedule when	|
	// | 								  | customer is not present. This is typically used for scenarios where		|
	// | 								  | customer stores credentials and makes a purchase on a given date and	|
	// |                                  | also set’s up a subscription.											|
	// | CUSTOMER_PRESENT_UNSCHEDULED     | Also known as (card-on-file) transactions. Payment details are stored	|
	// | 								  | to enable checkout with one-click, or simply to streamline the checkout |
	// | 								  | process. For card transaction customers typically do not enter a CVC	|
	// | 								  | number as part of the Checkout process.									|
	// | CUSTOMER_NOT_PRESENT_UNSCHEDULED | Unscheduled payments that are not recurring on a predefined schedule	|
	// | 								  | (e.g. balance top-up).													|
	// | MAIL_ORDER_TELEPHONE_ORDER       | Payments that are initiated by the customer via the merchant by mail 	|
	// | 								  | or telephone.															|
	// --------------------------------------------------------------------------------------------------------------
	PaymentMethod struct {
		PayerSelected  string `json:"payer_selected, omitempty"`  //default: PAYPAL
		PayeePreferred string `json:"payee_preferred, omitempty"` //default: UNRESTRICTED
		Category       string `json:"category, omitempty"`        //default: CUSTOMER_PRESENT_SINGLE_PURCHASE
	}

	// CreateSubscriptionRequest represents body parameters needed to create PayPal subscription
	CreateSubscriptionRequest struct {
		PlanID             string              `json:"plan_id"`
		StartTime          string              `json:"start_time, omitempty"` //default: current time
		Quantity           string              `json:"quantity, omitempty"`
		ShippingAmount     *Money              `json:"shipping_amount, omitempty"`
		Subscriber         *SubscriberRequest  `json:"subscriber, omitempty"`
		AutoRenewal        bool                `json:"auto_renewal, omitempty"`
		ApplicationContext *ApplicationContext `json:"application_context, omitempty"`
	}

	// SubscriberRequest represents the subscriber details
	SubscriberRequest struct {
		Name            *PayerName      `json:"name, omitempty, omitempty"`
		EmailAddress    string          `json:"email_address, omitempty"`
		PayerID         string          `json:"payer_id, omitempty"` //Read only
		ShippingAddress *ShippingDetail `json:"shipping_address, omitempty"`
		PaymentSource   *PaymentSource  `json:"payment_source, omitempty"`
	}

	// Subscriber represents the subscriber details
	Subscriber struct {
		Name            *Name                  `json:"name, omitempty, omitempty"`
		EmailAddress    string                 `json:"email_address, omitempty"`
		PayerID         string                 `json:"payer_id, omitempty"` //Read only
		ShippingAddress *ShippingDetail        `json:"shipping_address, omitempty"`
		PaymentSource   *PaymentSourceResponse `json:"payment_source, omitempty"`
	}

	// Name represents payer details
	Name struct {
		Prefix            string `json:"prefix, omitempty"`
		GivenName         string `json:"given_name, omitempty"`
		Surname           string `json:"surname, omitempty"`
		MiddleName        string `json:"middle_name, omitempty"`
		Suffix            string `json:"suffix, omitempty"`
		AlternateFullName string `json:"alternate_full_name, omitempty"`
		FullName          string `json:"full_name, omitempty"`
	}

	// PaymentSourceResponse represents the payment source definitions
	PaymentSourceResponse struct {
		Card *CardResponseWithBillingAddress `json:"card"`
	}

	// CardResponseWithBillingAddress represents card details
	// Brand represents the card brand or network. Typically used in the response. The possible values are:
	// ------------------------------------------------------
	// | VISA			 | Visa card. 						|
	// | MASTERCARD		 | Mastecard card. 					|
	// | DISCOVER		 | Discover card. 					|
	// | AMEX			 | American Express card. 			|
	// | SOLO			 | Solo debit card. 				|
	// | JCB			 | Japan Credit Bureau card. 		|
	// | STAR			 | Military Star card. 				|
	// | DELTA			 | Delta Airlines card. 			|
	// | SWITCH			 | Switch credit card. 				|
	// | MAESTRO 		 | Maestro credit card. 			|
	// | CB_NATIONALE	 | Carte Bancaire (CB) credit card. |
	// | CONFIGOGA		 | Configoga credit card. 			|
	// | CONFIDIS 		 | Confidis credit card. 			|
	// | ELECTRON 		 | Visa Electron credit card.		|
	// | CETELEM 		 | Cetelem credit card. 			|
	// | CHINA_UNION_PAY | China union pay credit card. 	|
	// ------------------------------------------------------
	// Type represents the payment card type. The possible values are:
	// ---------------------------------------------
	// | CREDIT  | A credit card. 				   |
	// | DEBIT   | A debit card. 				   |
	// | PREPAID | A Prepaid card. 				   |
	// | UNKNOWN | Card type cannot be determined. |
	// ---------------------------------------------
	CardResponseWithBillingAddress struct {
		LastDigit      string           `json:"last_digit, omitempty"` //Read only
		Brand          string           `json:"brand, omitempty"`      //Read only
		Type           string           `json:"type, omitempty"`       //Read only
		Name           string           `json:"name, omitempty"`
		BillingAddress *AddressPortable `json:"billing_address, omitempty"`
	}

	// PayerName represents payer name details
	PayerName struct {
		GivenName string `json:"given_name, omitempty"`
		Surname   string `json:"surname, omitempty"`
	}

	// Subscription represents the subscription details
	Subscription struct {
		ID               string                   `json:"id, omitempty"`
		Status           string                   `json:"status, omitempty"`
		StatusChangeNote string                   `json:"status_change_note, omitempty"`
		StatusUpdateTime string                   `json:"status_update_time, omitempty"`
		PlanID           string                   `json:"plan_id, omitempty"`
		StartTime        string                   `json:"start_time, omitempty"`
		Quantity         string                   `json:"quantity, omitempty"`
		ShippingAmount   *Money                   `json:"shipping_amount, omitempty"`
		Subscriber       *Subscriber              `json:"subscriber, omitempty"`
		BillingInfo      *SubscriptionBillingInfo `json:"billing_info, omitempty"` //Read only
		CreateTime       string                   `json:"created_time"`            //Read only
		UpdateTime       string                   `json:"update_time"`             //Read only
		Links            []*Link                  `json:"links"`                   //Read only
	}

	// SubscriptionBillingInfo represents billing details for subscription
	// The number of consecutive payment failures. Resets to 0 after a successful payment. If this reaches the payment_failure_threshold value,
	// the subscription updates to the SUSPENDED state.
	SubscriptionBillingInfo struct {
		OutstandingBalance  *Money               `json:"outstanding_balance"`
		CycleExecutions     []*CycleExecution    `json:"cycle_executions, omitempty"` //Read only
		LastPayment         LastPaymentDetails   `json:"last_payment, omitempty"`     //Read only
		NextBillingTime     string               `json:"next_billing_time"`           //Read only
		FinalPaymentTime    string               `json:"final_payment_time"`          //Read only
		FailedPaymentsCount uint64               `json:"failed_payments_count"`       //min: 0, max: 999
		LastFailedPayment   FailedPaymentDetails `json:"last_failed_payment"`         //Read only
	}

	// CycleExecution represents details about billing cycles executions
	// The number of times this billing cycle runs. Trial billing cycles can only have a value of 1 for total_cycles. Regular billing cycles
	// can either have infinite cycles (value of 0 for total_cycles) or a finite number of cycles (value between 1 and 999 for total_cycles).
	// TenureType represents the type of the billing cycle. The possible values are:
	// --------------------------------------
	// | REGULAR | A regular billing cycle. |
	// | TRIAL   | A trial billing cycle.   |
	// --------------------------------------
	CycleExecution struct {
		TenureType                  string `json:"tenure_type"`                               //Read only
		Sequence                    uint64 `json:"sequence"`                                  //min: 0, max: 99
		CyclesCompleted             uint64 `json:"cycles_completed"`                          //min: 0, max: 9999 Read only
		CyclesRemaining             uint64 `json:"cycles_remaining, omitempty"`               //min: 0, max: 9999 Read only
		CurrentPricingSchemeVersion uint64 `json:"current_pricing_scheme_version, omitempty"` //min: 0, max: 99 Read only
		TotalCycles                 uint64 `json:"total_cycles, omitempty"`                   //min: 0, max: 999 Read only
	}

	// LastPaymentDetails represents details for the last payment
	LastPaymentDetails struct {
		Amount *Money `json:"amount"` //Read only
		Time   string `json:"time"`   //Read only
	}

	// FailedPaymentDetails represents details about failed payment
	// ReasonCode represents the reason code for the payment failure. The possible values are:
	// -----------------------------------------------------------------------------------------------------------------
	// | PAYMENT_DENIED 		 			  | PayPal declined the payment due to one or more customer issues.        |
	// | INTERNAL_SERVER_ERROR   			  | An internal server error has occurred.								   |
	// | PAYEE_ACCOUNT_RESTRICTED			  | The payee account is not in good standing and cannot receive payments. |
	// | PAYER_ACCOUNT_RESTRICTED			  | The payer account is not in good standing and cannot make payments.	   |
	// | PAYER_CANNOT_PAY        			  | Payer cannot pay for this transaction. 								   |
	// | SENDING_LIMIT_EXCEEDED  			  | The transaction exceeds the payer's sending limit.                     |
	// | TRANSACTION_RECEIVING_LIMIT_EXCEEDED | The transaction exceeds the receiver's receiving limit.				   |
	// | CURRENCY_MISMATCH                    | The transaction is declined due to a currency mismatch.				   |
	// -----------------------------------------------------------------------------------------------------------------
	FailedPaymentDetails struct {
		Amount               *Money `json:"amount"`                             //Read only
		Time                 string `json:"time"`                               //Read only
		ReasonCode           string `json:"reason_code, omitempty"`             //Read only
		NextPaymentRetryTime string `json:"next_payment_retry_time, omitempty"` //Read only
	}

	// ShowSubscriptionRequest represents query parameters for show subscription call
	// Fields represents list of fields that are to be returned in the response.
	// Possible value for fields is last_failed _payment.
	ShowSubscriptionRequest struct {
		Fields string `json:"fields, omitempty"`
	}

	// UpdateSubscriptionStatusRequest represents body parameters for activate subscription
	UpdateSubscriptionStatusRequest struct {
		Reason string `json:"reason"`
	}

	// CaptureAuthorizedPaymentOnSubscriptionRequest represents body parameter for capturing authorized payment on subscription
	// CaptureType represents the type of capture. The allowed values are:
	// ---------------------------------------------------------------------------------
	// | OUTSTANDING_BALANCE | The outstanding balance that the subscriber must clear. |
	// ---------------------------------------------------------------------------------
	CaptureAuthorizedPaymentOnSubscriptionRequest struct {
		Note        string `json:"note"`
		CaptureType string `json:"capture_type"`
		Amount      *Money `json:"amount"`
	}

	// ListTransactionsForSubscriptionRequest represents query parameters for list transactions for subscription
	ListTransactionsForSubscriptionRequest struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}

	//TransactionsList represents list of transactions
	TransactionsList struct {
		Transactions []*Transaction `json:"transactions, omitempty"`
		TotalItems   uint64         `json:"total_items, omitempty"`
		TotalPages   uint64         `json:"total_pages, omitempty"`
		Links        []*Link        `json:"links, omitempty"` //Read only
	}

	// ReviseSubscriptionRequest represents body parameters for revise subscription
	// (update quantity of product or service in subscription)
	ReviseSubscriptionRequest struct {
		PlanID             string              `json:"plan_id, omitempty"`
		Quantity           string              `json:"quantity, omitempty"`
		ShippingAmount     *Money              `json:"shipping_amount, omitempty"`
		ShippingAddress    *ShippingDetail     `json:"shipping_address, omitempty"`
		ApplicationContext *ApplicationContext `json:"application_context, omitempty"`
	}

	// ReviseSubscriptionResponse represents response from revise subscription
	// (update quantity of product or service in subscription)
	ReviseSubscriptionResponse struct {
		PlanID          string          `json:"plan_id, omitempty"`
		Quantity        string          `json:"quantity, omitempty"`
		EffectiveTime   string          `json:"effective_time, omitempty"` //Read only
		ShippingAmount  *Money          `json:"shipping_amount, omitempty"`
		ShippingAddress *ShippingDetail `json:"shipping_address, omitempty"`
		Links           []*Link         `json:"links, omitempty"` //Read only
	}

	// Event represents hook response
	// The possible values for Resource are:
	// -----------------------------------------------
	// | Hook Name  				  | Object 		 |
	// -----------------------------------------------
	// | CATALOG.PRODUCT.CREATED 	  | Product 	 |
	// | BILLING.PLAN.CREATED 		  | Plan 		 |
	// | BILLING.SUBSCRIPTION.CREATED | Subscription |
	// -----------------------------------------------
	Event struct {
		ID              string          `json:"id"`
		EventVersion    string          `json:"event_version"`
		CreateTime      string          `json:"create_time"`
		ResourceType    string          `json:"resource_type"`
		ResourceVersion string          `json:"resource_version"`
		EventType       string          `json:"event_type"`
		Summary         string          `json:"summary"`
		Resource        json.RawMessage `json:"resource"`
		Links           []*Link         `json:"links"`
	}
)

// Error method implementation for ErrorResponse struct
func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d %s, %+v", r.Response.Request.Method, r.Response.Request.URL, r.Response.StatusCode, r.Message, r.Details)
}

// MarshalJSON for JSONTime
func (t JSONTime) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf(`"%s"`, time.Time(t).UTC().Format(time.RFC3339))
	return []byte(stamp), nil
}

func (e *expirationTime) UnmarshalJSON(b []byte) error {
	var n json.Number
	err := json.Unmarshal(b, &n)
	if err != nil {
		return err
	}
	i, err := n.Int64()
	if err != nil {
		return err
	}
	*e = expirationTime(i)
	return nil
}
