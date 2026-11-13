package calc

import "strings"

// fiatCodes are the ISO 4217 currencies Gocast converts (no metals, no
// crypto: Frankfurter also serves those).
const fiatCodes = "AED AFN ALL AMD ANG AOA ARS AUD AWG AZN BAM BBD BDT BGN BHD BIF BMD BND BOB BRL BSD BTN BWP BYN BZD " +
	"CAD CDF CHF CLP CNY COP CRC CUP CVE CZK DJF DKK DOP DZD EGP ERN ETB EUR FJD FKP GBP GEL GHS GIP GMD GNF GTQ GYD " +
	"HKD HNL HTG HUF IDR ILS INR IQD IRR ISK JMD JOD JPY KES KGS KHR KMF KPW KRW KWD KYD KZT LAK LBP LKR LRD LSL LYD " +
	"MAD MDL MGA MKD MMK MNT MOP MRU MUR MVR MWK MXN MYR MZN NAD NGN NIO NOK NPR NZD OMR PAB PEN PGK PHP PKR PLN PYG " +
	"QAR RON RSD RUB RWF SAR SBD SCR SDG SEK SGD SHP SLE SLL SOS SRD SSP STN SVC SYP SZL THB TJS TMT TND TOP TRY TTD " +
	"TWD TZS UAH UGX USD UYU UZS VES VND VUV WST XAF XCD XOF XPF YER ZAR ZMW ZWG ZWL"

// zeroDecimals are the currencies written without minor units. The brief
// names JPY and KRW; the others have none in ISO 4217 either.
var zeroDecimals = map[string]bool{
	"JPY": true, "KRW": true, "VND": true, "CLP": true, "ISK": true, "PYG": true, "UGX": true,
	"XAF": true, "XOF": true, "XPF": true, "KMF": true, "GNF": true, "RWF": true, "VUV": true,
	"DJF": true, "BIF": true,
}

// currencyNames are the names, symbols and Chinese words for currencies,
// besides their codes. "¥" is ¥ (CNY or JPY by region).
var currencyNames = map[string]string{
	"USD": "$|US$|dollar|dollars|us dollar|us dollars|buck|bucks|美元|美金|美刀",
	"CNY": "rmb|yuan|renminbi|chinese yuan|人民币|元|块|块钱|圆",
	"JPY": "円|yen|japanese yen|日元|日币|日圆",
	"EUR": "€|euro|euros|欧元",
	"GBP": "£|pound|pounds|british pound|british pounds|pound sterling|sterling|英镑",
	"HKD": "HK$|hong kong dollar|hong kong dollars|港币|港元|港纸",
	"TWD": "NT$|taiwan dollar|taiwan dollars|new taiwan dollar|新台币|台币",
	"KRW": "₩|won|korean won|韩元|韩币",
	"AUD": "A$|AU$|australian dollar|australian dollars|澳元|澳币|澳大利亚元",
	"CAD": "C$|CA$|canadian dollar|canadian dollars|加元|加币|加拿大元",
	"SGD": "S$|singapore dollar|singapore dollars|新加坡元|新币",
	"NZD": "NZ$|new zealand dollar|new zealand dollars|新西兰元|纽币",
	"CHF": "swiss franc|swiss francs|franc|francs|瑞士法郎|瑞郎|法郎",
	"RUB": "₽|ruble|rubles|rouble|roubles|卢布|俄罗斯卢布",
	"INR": "₹|rupee|rupees|indian rupee|indian rupees|卢比|印度卢比",
	"THB": "฿|baht|泰铢",
	"VND": "₫|dong|越南盾",
	"MOP": "MOP$|pataca|patacas|澳门元|澳门币|葡币",
	"MYR": "RM|ringgit|林吉特|令吉|马币",
	"PHP": "₱|philippine peso|philippine pesos|菲律宾比索",
	"MXN": "MX$|peso|pesos|mexican peso|mexican pesos|墨西哥比索",
	"IDR": "rupiah|印尼盾",
	"BRL": "R$|real|reais|brazilian real|雷亚尔|巴西雷亚尔",
	"ZAR": "rand|南非兰特|兰特",
	"TRY": "₺|lira|turkish lira|土耳其里拉|里拉",
	"SEK": "swedish krona|瑞典克朗",
	"NOK": "norwegian krone|挪威克朗",
	"DKK": "danish krone|丹麦克朗",
	"ILS": "₪|shekel|shekels|谢克尔|新谢克尔",
	"AED": "dirham|dirhams|迪拉姆",
	"SAR": "riyal|riyals|沙特里亚尔",
	"PLN": "zloty|兹罗提",
	"HUF": "forint|福林",
	"UAH": "₴|hryvnia|格里夫纳",
}

// currencySymbols are the one-character symbols the lexer reads as
// currency tokens (plus letters followed by "$", like HK$).
var currencySymbols = map[rune]bool{
	'$': true, '€': true, '£': true, '¥': true, '₩': true, '₹': true, '₽': true,
	'฿': true, '₫': true, '₱': true, '₺': true, '₪': true, '₴': true,
}

var fiat = map[string]bool{}

func initCurrencies() {
	for _, c := range strings.Fields(fiatCodes) {
		fiat[c] = true
		u := &unit{code: c, cat: catCurrency}
		units[c] = u
		addName(strings.ToLower(c), entry{u: u})
	}
	for c, ns := range currencyNames {
		for _, n := range strings.Split(ns, "|") {
			addName(n, entry{u: units[c]})
		}
	}
	addName("¥", entry{yen: true})
}

// IsCurrency reports whether code is a currency Gocast converts.
func IsCurrency(code string) bool { return fiat[code] }

// CurrencyDecimals is how many fraction digits an amount in code shows.
func CurrencyDecimals(code string) int {
	if zeroDecimals[code] {
		return 0
	}
	return 2
}

// regionCurrency maps ISO 3166 region codes to their currency.
var regionCurrency = map[string]string{
	"US": "USD", "PR": "USD", "EC": "USD", "SV": "USD", "PA": "USD", "GU": "USD",
	"CN": "CNY", "JP": "JPY", "KR": "KRW", "TW": "TWD", "HK": "HKD", "MO": "MOP",
	"SG": "SGD", "MY": "MYR", "TH": "THB", "VN": "VND", "ID": "IDR", "PH": "PHP",
	"IN": "INR", "PK": "PKR", "BD": "BDT", "LK": "LKR", "NP": "NPR", "KH": "KHR",
	"LA": "LAK", "MM": "MMK", "MN": "MNT", "KZ": "KZT", "UZ": "UZS",
	"AU": "AUD", "NZ": "NZD", "CA": "CAD", "MX": "MXN", "BR": "BRL", "AR": "ARS",
	"CL": "CLP", "CO": "COP", "PE": "PEN", "UY": "UYU", "PY": "PYG", "BO": "BOB",
	"VE": "VES", "CR": "CRC", "GT": "GTQ", "DO": "DOP", "JM": "JMD",
	"GB": "GBP", "CH": "CHF", "LI": "CHF", "NO": "NOK", "SE": "SEK", "DK": "DKK",
	"IS": "ISK", "PL": "PLN", "CZ": "CZK", "HU": "HUF", "RO": "RON", "RS": "RSD",
	"UA": "UAH", "RU": "RUB", "BY": "BYN", "TR": "TRY", "GE": "GEL", "AM": "AMD",
	"AZ": "AZN", "MD": "MDL", "MK": "MKD", "BA": "BAM", "AL": "ALL",
	"IL": "ILS", "AE": "AED", "SA": "SAR", "QA": "QAR", "KW": "KWD", "BH": "BHD",
	"OM": "OMR", "JO": "JOD", "EG": "EGP", "MA": "MAD", "TN": "TND", "DZ": "DZD",
	"ZA": "ZAR", "NG": "NGN", "KE": "KES", "GH": "GHS", "ET": "ETB", "TZ": "TZS",
	"UG": "UGX",
	// Euro area.
	"AT": "EUR", "BE": "EUR", "BG": "EUR", "CY": "EUR", "DE": "EUR", "EE": "EUR",
	"ES": "EUR", "FI": "EUR", "FR": "EUR", "GR": "EUR", "HR": "EUR", "IE": "EUR",
	"IT": "EUR", "LT": "EUR", "LU": "EUR", "LV": "EUR", "MT": "EUR", "NL": "EUR",
	"PT": "EUR", "SI": "EUR", "SK": "EUR", "AD": "EUR", "MC": "EUR", "SM": "EUR",
	"VA": "EUR", "ME": "EUR", "XK": "EUR",
}

// CurrencyForRegion is the currency of an ISO 3166 region code ("JP" →
// "JPY"); USD when the region is unknown.
func CurrencyForRegion(region string) string {
	if c, ok := regionCurrency[strings.ToUpper(region)]; ok {
		return c
	}
	return "USD"
}

// yenFor is what ¥ means in region: JPY in Japan, CNY elsewhere.
func yenFor(region string) string {
	if strings.EqualFold(region, "JP") {
		return "JPY"
	}
	return "CNY"
}
