package atlas

// englishBooks holds the English names and common abbreviations of the 66
// books, by book number. The atlas names its books in Spanish only, but
// agents and users write "Acts 16:1" and "2 Kings 3". These forms are added
// after every Spanish form of every book, so a Spanish form always wins a
// clash; TestEnglishBookNames checks that none clashes today.
var englishBooks = map[int][]string{
	1: {"Genesis", "Gen", "Gn"}, 2: {"Exodus", "Exod"}, 3: {"Leviticus", "Lev", "Lv"},
	4: {"Numbers", "Num"}, 5: {"Deuteronomy", "Deut", "Dt"}, 6: {"Joshua", "Josh"},
	7: {"Judges", "Judg"}, 8: {"Ruth"},
	9: {"1 Samuel", "1 Sam", "First Samuel", "I Samuel"}, 10: {"2 Samuel", "2 Sam", "Second Samuel", "II Samuel"},
	11: {"1 Kings", "1 Kgs", "First Kings", "I Kings"}, 12: {"2 Kings", "2 Kgs", "Second Kings", "II Kings"},
	13: {"1 Chronicles", "1 Chr", "1 Chron", "First Chronicles", "I Chronicles"},
	14: {"2 Chronicles", "2 Chr", "2 Chron", "Second Chronicles", "II Chronicles"},
	15: {"Ezra"}, 16: {"Nehemiah"}, 17: {"Esther", "Esth"}, 18: {"Job"},
	19: {"Psalms", "Psalm", "Ps", "Pss", "Psa"}, 20: {"Proverbs", "Prov"},
	21: {"Ecclesiastes", "Eccl", "Eccles"}, 22: {"Song of Solomon", "Song of Songs", "Song"},
	23: {"Isaiah", "Isa"}, 24: {"Jeremiah"}, 25: {"Lamentations"}, 26: {"Ezekiel", "Ezek"},
	27: {"Daniel"}, 28: {"Hosea", "Hos"}, 29: {"Joel"}, 30: {"Amos"}, 31: {"Obadiah", "Obad"},
	32: {"Jonah"}, 33: {"Micah", "Mic"}, 34: {"Nahum"}, 35: {"Habakkuk"}, 36: {"Zephaniah", "Zeph"},
	37: {"Haggai", "Hag"}, 38: {"Zechariah", "Zech"}, 39: {"Malachi"},
	40: {"Matthew", "Matt"}, 41: {"Mark", "Mk"}, 42: {"Luke", "Lk"}, 43: {"John"},
	44: {"Acts", "Acts of the Apostles"}, 45: {"Romans"},
	46: {"1 Corinthians", "First Corinthians", "I Corinthians"}, 47: {"2 Corinthians", "Second Corinthians", "II Corinthians"},
	48: {"Galatians"}, 49: {"Ephesians", "Eph"}, 50: {"Philippians", "Phil", "Php"}, 51: {"Colossians"},
	52: {"1 Thessalonians", "1 Thess", "First Thessalonians", "I Thessalonians"},
	53: {"2 Thessalonians", "2 Thess", "Second Thessalonians", "II Thessalonians"},
	54: {"1 Timothy", "First Timothy", "I Timothy"}, 55: {"2 Timothy", "Second Timothy", "II Timothy"},
	56: {"Titus"}, 57: {"Philemon", "Phlm", "Philem"}, 58: {"Hebrews"}, 59: {"James", "Jas"},
	60: {"1 Peter", "1 Pet", "First Peter", "I Peter"}, 61: {"2 Peter", "2 Pet", "Second Peter", "II Peter"},
	62: {"1 John", "First John", "I John"}, 63: {"2 John", "Second John", "II John"}, 64: {"3 John", "Third John", "III John"},
	65: {"Jude"}, 66: {"Revelation", "Rev", "Apocalypse"},
}

func englishKeys(num int) []string {
	names := englishBooks[num]
	keys := make([]string, 0, len(names))
	for _, n := range names {
		keys = append(keys, BookKey(n))
	}
	return keys
}
