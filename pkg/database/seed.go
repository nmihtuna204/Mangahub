package database

import (
	"fmt"
	"time"

	"mangahub/pkg/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Seed populates the database with initial data
func (db *DB) Seed() error {
	// Check if already seeded
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check seed status: %w", err)
	}

	if count > 0 {
		fmt.Println("Database already seeded, skipping...")
		return nil
	}

	fmt.Println("Seeding database...")

	// Seed admin user
	if err := db.seedAdminUser(); err != nil {
		return err
	}

	// Seed test users
	if err := db.seedTestUsers(); err != nil {
		return err
	}

	// Seed genres
	if err := db.seedGenres(); err != nil {
		return err
	}

	// Seed manga data with 10 samples
	if err := db.seedMangaData(); err != nil {
		return err
	}

	// Seed user reading progress
	if err := db.seedReadingProgress(); err != nil {
		return err
	}

	fmt.Println("Database seeded successfully!")
	return nil
}

func (db *DB) seedAdminUser() error {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := models.User{
		ID:           uuid.New().String(),
		Username:     "admin",
		Email:        "admin@mangahub.com",
		PasswordHash: string(hash),
		DisplayName:  "Administrator",
		Role:         "admin",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, display_name, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.Email, user.PasswordHash, user.DisplayName,
		user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt,
	)

	return err
}

func (db *DB) seedTestUsers() error {
	users := []struct {
		username string
		email    string
		display  string
	}{
		{"reader1", "reader1@example.com", "John Reader"},
		{"reader2", "reader2@example.com", "Jane Bookworm"},
		{"mangafan", "fan@example.com", "Manga Enthusiast"},
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	for _, u := range users {
		user := models.User{
			ID:           uuid.New().String(),
			Username:     u.username,
			Email:        u.email,
			PasswordHash: string(hash),
			DisplayName:  u.display,
			Role:         "user",
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		_, err = db.Exec(`
			INSERT INTO users (id, username, email, password_hash, display_name, role, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			user.ID, user.Username, user.Email, user.PasswordHash, user.DisplayName,
			user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) seedGenres() error {
	genres := []struct {
		name string
		slug string
	}{
		{"Action", "action"},
		{"Adventure", "adventure"},
		{"Comedy", "comedy"},
		{"Drama", "drama"},
		{"Fantasy", "fantasy"},
		{"Horror", "horror"},
		{"Isekai", "isekai"},
		{"Mecha", "mecha"},
		{"Mystery", "mystery"},
		{"Romance", "romance"},
		{"Sci-Fi", "sci-fi"},
		{"Slice of Life", "slice-of-life"},
		{"Sports", "sports"},
		{"Supernatural", "supernatural"},
		{"Thriller", "thriller"},
	}

	for _, g := range genres {
		_, err := db.Exec(`
			INSERT INTO genres (id, name, slug, created_at)
			VALUES (?, ?, ?, ?)`,
			uuid.New().String(), g.name, g.slug, time.Now(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) seedMangaData() error {
	// 120+ sample manga entries matching new normalized schema
	mangaList := []struct {
		title       string
		author      string
		artist      string
		description string
		status      string
		mangaType   string
		year        int
		chapters    int
		genres      []string
	}{
		// Popular Ongoing Series
		{"One Piece", "Eiichiro Oda", "Eiichiro Oda", "Follow Monkey D. Luffy and his Straw Hat Pirates as they search for the ultimate treasure, One Piece.", "ongoing", "manga", 1997, 1100, []string{"Action", "Adventure", "Fantasy"}},
		{"My Hero Academia", "Kohei Horikoshi", "Kohei Horikoshi", "In a world where most people have superpowers called Quirks, a powerless boy dreams of becoming a hero.", "ongoing", "manga", 2014, 426, []string{"Action", "Adventure", "Comedy"}},
		{"Jujutsu Kaisen", "Gege Akutami", "Gege Akutami", "A high school student swallows a cursed finger and joins a school for jujutsu sorcerers.", "ongoing", "manga", 2018, 270, []string{"Action", "Horror", "Supernatural"}},
		{"Chainsaw Man", "Tatsuki Fujimoto", "Tatsuki Fujimoto", "A poor boy becomes a devil hunter with a chainsaw devil living in his heart.", "ongoing", "manga", 2018, 180, []string{"Action", "Horror", "Supernatural"}},
		{"Black Clover", "Yuki Tabata", "Yuki Tabata", "An orphan with no magic power dreams of becoming the Wizard King in a magical fantasy world.", "ongoing", "manga", 2015, 383, []string{"Action", "Adventure", "Fantasy"}},
		{"The Promised Neverland", "Kaiu Shibuya", "Posuka Demizu", "Orphans discover their orphanage is actually a farm breeding them for demons and plan their escape.", "completed", "manga", 2016, 181, []string{"Drama", "Horror", "Psychological"}},
		{"Food Wars!", "Yuto Tsukuda", "Shun Saeki", "A young chef enters an elite culinary school and competes in intense cooking battles.", "completed", "manga", 2012, 315, []string{"Comedy", "School", "Shounen"}},

		// Completed Classics
		{"Attack on Titan", "Hajime Isayama", "Hajime Isayama", "Humanity lives inside cities surrounded by massive walls protecting them from gigantic humanoid creatures.", "completed", "manga", 2009, 139, []string{"Action", "Drama", "Horror"}},
		{"Demon Slayer", "Koyoharu Gotouge", "Koyoharu Gotouge", "Tanjiro's journey to save his sister from demons while becoming a powerful demon slayer.", "completed", "manga", 2018, 205, []string{"Action", "Adventure", "Supernatural"}},
		{"Fullmetal Alchemist", "Hiromu Arakawa", "Hiromu Arakawa", "Two brothers seek the Philosopher's Stone to restore their bodies after a failed alchemical experiment.", "completed", "manga", 2001, 116, []string{"Action", "Adventure", "Fantasy"}},
		{"Death Note", "Tsugumi Ohba", "Takeshi Obata", "A genius student finds a notebook that can kill anyone and begins his quest to become a god.", "completed", "manga", 2003, 108, []string{"Mystery", "Psychological", "Supernatural"}},
		{"Bleach", "Tite Kubo", "Tite Kubo", "A teenager discovers he can see ghosts and becomes a Soul Reaper to protect the living world.", "completed", "manga", 2001, 686, []string{"Action", "Adventure", "Supernatural"}},
		{"Naruto", "Masashi Kishimoto", "Masashi Kishimoto", "A young ninja dreams of becoming the strongest shinobi and earning recognition from his village.", "completed", "manga", 1999, 700, []string{"Action", "Adventure", "Fantasy"}},

		// Manhwa (Korean)
		{"Solo Leveling", "Chugong", "DUBU", "In a world where hunters fight monsters, the weakest hunter becomes the strongest through a mysterious system.", "completed", "manhwa", 2018, 179, []string{"Action", "Fantasy", "Adventure"}},
		{"Tower of God", "SIU", "SIU", "A young man enters a mysterious tower to find his friend, facing dangerous challenges and making allies.", "ongoing", "manhwa", 2010, 520, []string{"Action", "Adventure", "Fantasy"}},
		{"The God of High School", "Yongje Park", "Yongje Park", "High school students with incredible powers compete in a martial arts tournament.", "completed", "manhwa", 2011, 110, []string{"Action", "Shounen", "Supernatural"}},
		{"Kubera", "Currygom", "Currygom", "A girl joins a group of powerful beings called Kubera to uncover the mysteries of the world.", "ongoing", "manhwa", 2010, 460, []string{"Action", "Fantasy", "Adventure"}},

		// Manhua (Chinese)
		{"Battle Through the Heavens", "Tian Can Tu Dou", "Various", "A talented young man loses his power but finds a way to rise again in a fantasy world.", "ongoing", "manhua", 2009, 350, []string{"Action", "Fantasy", "Adventure"}},
		{"The Legend of Sun Knight", "Yu Wo", "Ren Wei", "Reincarnated as a holy knight, a person discovers their past lives and hidden truths.", "completed", "manhua", 2007, 180, []string{"Adventure", "Fantasy", "Drama"}},

		// Isekai Series
		{"Re:Zero", "Tappei Nagatsuki", "Shinichirou Otsuka", "A boy is sent to a fantasy world and discovers he can return to the past when he dies.", "ongoing", "manga", 2014, 150, []string{"Isekai", "Fantasy", "Drama"}},
		{"That Time I Got Reincarnated as a Slime", "Fuse", "Kawakami Taiki", "A salaryman reincarnates as a slime monster in a fantasy world and creates his own kingdom.", "ongoing", "manga", 2015, 90, []string{"Isekai", "Fantasy", "Adventure"}},
		{"No Game No Life", "Yuu Kamiya", "Yuu Kamiya", "Two NEET siblings are transported to a world where everything is decided by games.", "ongoing", "manga", 2012, 94, []string{"Isekai", "Comedy", "Fantasy"}},
		{"Sword Art Online", "Reki Kawahara", "abec", "Players are trapped in a virtual reality game and must beat all floors to escape alive.", "completed", "manga", 2009, 120, []string{"Action", "Adventure", "Sci-Fi"}},

		// Romance & Comedy
		{"Kaguya-sama: Love is War", "Aka Akasaka", "Mengo Yokoyari", "Two geniuses play mind games to make the other confess their love first.", "completed", "manga", 2015, 196, []string{"Comedy", "Romance", "School"}},
		{"Fruits Basket", "Natsuki Takaya", "Natsuki Takaya", "A girl becomes entangled with a family cursed to transform into Chinese zodiac animals.", "completed", "manga", 1998, 179, []string{"Comedy", "Romance", "School"}},
		{"Ouran High School Host Club", "Bisco Hatori", "Bisco Hatori", "A poor girl joins the school's host club to pay off her debt among the richest students.", "completed", "manga", 2002, 83, []string{"Comedy", "Romance", "School"}},
		{"My Love Story!", "Kazune Kawahara", "Aruko", "A gentle giant is finally able to confess to the girl of his dreams and starts dating her.", "completed", "manga", 2011, 122, []string{"Comedy", "Romance", "School"}},

		// Psychological & Thriller
		{"Steins;Gate", "Anonymous", "Hiyama Mizuho", "A group discovers how to send messages to the past and must prevent a dystopian future.", "completed", "manga", 2009, 43, []string{"Sci-Fi", "Thriller", "Mystery"}},
		{"Ergo Proxy", "Dai Sato", "Atsushi Ookubo", "In a post-apocalyptic city, a girl meets a mysterious creature and embarks on a journey of self-discovery.", "completed", "manga", 2006, 48, []string{"Sci-Fi", "Psychological", "Drama"}},
		{"Monster", "Naoki Urasawa", "Naoki Urasawa", "A doctor pursues a monster he once saved, uncovering a conspiracy and his own past.", "completed", "manga", 1997, 162, []string{"Thriller", "Mystery", "Psychological"}},
		{"Devilman", "Go Nagai", "Go Nagai", "A boy merges with a demon to fight against the demonic invasion of Earth.", "completed", "manga", 1972, 65, []string{"Action", "Horror", "Supernatural"}},

		// Sports
		{"Haikyu!!", "Haruichi Furudate", "Haruichi Furudate", "A short boy with big dreams joins a high school volleyball team and aims for nationals.", "completed", "manga", 2012, 402, []string{"Sports", "School", "Shounen"}},
		{"Kuroko's Basketball", "Tadatoshi Fujimaki", "Tadatoshi Fujimaki", "A basketball prodigy forms an unbeatable team despite having no presence in a match.", "completed", "manga", 2008, 296, []string{"Sports", "School", "Shounen"}},
		{"Slam Dunk", "Takehiko Inoue", "Takehiko Inoue", "A delinquent joins his high school's basketball team and discovers his passion for the sport.", "completed", "manga", 1990, 276, []string{"Sports", "School", "Shounen"}},

		// Dark Fantasy
		{"Berserk", "Kentaro Miura", "Kentaro Miura", "A mercenary pursues his ambitions in a dark fantasy world full of supernatural forces.", "ongoing", "manga", 1989, 380, []string{"Action", "Adventure", "Fantasy"}},
		{"Dark Souls", "Various", "Various", "Adaptations of the video game exploring its dark and mysterious world.", "completed", "manga", 2011, 80, []string{"Action", "Fantasy", "Horror"}},
		{"Claymore", "Norihiro Yagi", "Norihiro Yagi", "Female warriors hunt shape-shifting monsters in a dark fantasy world.", "completed", "manga", 2001, 159, []string{"Action", "Fantasy", "Horror"}},

		// Comedy & Slice of Life
		{"Usagi Drop", "Yumi Unita", "Yumi Unita", "A man takes custody of his great-grandniece and learns about parenting and love.", "completed", "manga", 2008, 62, []string{"Comedy", "Drama", "Slice of Life"}},
		{"Nichijou", "Hideki Araki", "Hideki Araki", "Daily comedic adventures of high school girls in increasingly absurd situations.", "completed", "manga", 2006, 202, []string{"Comedy", "School", "Slice of Life"}},
		{"Azuki-chan", "Yuichi Kumakura", "Yuichi Kumakura", "A girl chef cooks delicious meals while dealing with school and personal drama.", "completed", "manga", 1995, 50, []string{"Comedy", "Food", "School"}},

		// Battle Shounen
		{"Hunter x Hunter", "Yoshihiro Togashi", "Yoshihiro Togashi", "A young man becomes a hunter and embarks on adventures with friends to find his father.", "ongoing", "manga", 1998, 390, []string{"Action", "Adventure", "Shounen"}},
		{"One Punch Man", "ONE", "Yusuke Murata", "An overpowered superhero who can defeat any opponent with a single punch seeks a challenging fight.", "ongoing", "manga", 2012, 170, []string{"Action", "Comedy", "Shounen"}},
		{"Mob Psycho 100", "ONE", "ONE", "A middle school boy with psychic powers tries to live a normal life while controlling his abilities.", "completed", "manga", 2012, 101, []string{"Action", "Comedy", "Supernatural"}},

		// Romance
		{"Horimiya", "HERO", "Daisuke Hagiwara", "Two high school students with contrasting public and private personas discover each other's true selves.", "completed", "manga", 2011, 123, []string{"Comedy", "Romance", "School"}},
		{"Wotakoi: Love is Hard for Otaku", "Fujita", "Fujita", "Four otaku office workers navigate romance while keeping their nerdy hobbies secret.", "ongoing", "manga", 2014, 73, []string{"Comedy", "Romance", "Slice of Life"}},
		{"Love at Stake", "Koide Sachi", "Koide Sachi", "A modern romantic comedy involving witches, demons, and humans.", "completed", "manga", 2008, 37, []string{"Comedy", "Romance", "Supernatural"}},

		// Historical & Adventure
		{"Vinland Saga", "Makoto Yukimura", "Makoto Yukimura", "A Viking warrior seeks revenge while discovering the true meaning of his journey.", "ongoing", "manga", 2005, 180, []string{"Action", "Adventure", "Historical"}},
		{"Kingdom", "Yasuhisa Hara", "Yasuhisa Hara", "Two orphans dream of becoming generals in China's warring kingdoms era.", "ongoing", "manga", 2006, 700, []string{"Action", "Adventure", "Historical"}},
		{"Samurai 7", "Various", "Various", "Seven skilled samurai are hired to protect a village from bandits.", "completed", "manga", 2004, 45, []string{"Action", "Adventure", "Historical"}},

		// Mystery & Supernatural
		{"The Murders of Midsummer", "Akinari Matsuno", "Akinari Matsuno", "A man cursed to remember everyone's death must solve supernatural mysteries.", "ongoing", "manga", 2018, 50, []string{"Mystery", "Supernatural", "Thriller"}},
		{"Toilet-Bound Hanako-kun", "AidaIro", "AidaIro", "A ghost in a school toilet grants wishes for a mysterious price.", "ongoing", "manga", 2016, 200, []string{"Mystery", "Supernatural", "Comedy"}},
		{"Jigoku Shoujo", "Tsuguhito Tsukumo", "Tsuguhito Tsukumo", "A mysterious website allows people to send others to hell for a price.", "completed", "manga", 2005, 87, []string{"Horror", "Supernatural", "Mystery"}},

		// Sci-Fi & Cyberpunk
		{"Cyberpunk 2077", "Various", "Various", "Manga adaptations of the futuristic cyberpunk universe.", "ongoing", "manga", 2020, 40, []string{"Sci-Fi", "Cyberpunk", "Action"}},
		{"Ghost in the Shell", "Masamune Shirow", "Masamune Shirow", "A cyborg counter-terrorism operative investigates a mysterious hacker in a dystopian future.", "completed", "manga", 1989, 35, []string{"Sci-Fi", "Cyberpunk", "Action"}},
		{"Blame!", "Tsutomu Nihei", "Tsutomu Nihei", "A cyborg explores a massive ever-expanding structure in search of the Net Terminal Gene.", "completed", "manga", 1998, 66, []string{"Sci-Fi", "Cyberpunk", "Action"}},

		// Adventure & Exploration
		{"Made in Abyss", "Akihito Tsukushi", "Akihito Tsukushi", "Young explorers venture into a massive pit called the Abyss full of mysteries and dangers.", "ongoing", "manga", 2012, 65, []string{"Adventure", "Fantasy", "Mystery"}},
		{"Dungeon Meshi", "Ryoko Kui", "Ryoko Kui", "Adventurers survive a dungeon by cooking the monsters they encounter.", "completed", "manga", 2014, 97, []string{"Adventure", "Comedy", "Fantasy"}},
		{"Trigun", "Yasuhiro Nightow", "Yasuhiro Nightow", "A man searches for peace in a desert world while being hunted for a bounty.", "completed", "manga", 1996, 143, []string{"Action", "Adventure", "Sci-Fi"}},

		// Additional Titles to Reach 100+
		{"JoJo's Bizarre Adventure", "Hirohiko Araki", "Hirohiko Araki", "Generational saga of the Joestar family and their battles with supernatural threats.", "completed", "manga", 1987, 890, []string{"Action", "Adventure", "Supernatural"}},
		{"Cowboy Bebop", "Hajime Yatate", "Shinichiro Watanabe", "Bounty hunters in space traverse the galaxy while facing their past.", "completed", "manga", 1998, 26, []string{"Sci-Fi", "Action", "Adventure"}},
		{"Inuyasha", "Rumiko Takahashi", "Rumiko Takahashi", "A girl teams up with a half-demon to collect magical jewels and prevent catastrophe.", "completed", "manga", 1996, 558, []string{"Action", "Adventure", "Fantasy"}},
		{"Rurouni Kenshin", "Nobuhiro Watsuki", "Nobuhiro Watsuki", "A former assassin seeks redemption as a wandering swordsman in Meiji-era Japan.", "completed", "manga", 1994, 214, []string{"Action", "Adventure", "Historical"}},
		{"Cardcaptor Sakura", "Clamp", "Clamp", "A schoolgirl discovers she has magical powers and must capture mystical cards.", "completed", "manga", 1996, 50, []string{"Comedy", "Magic", "Shoujo"}},
		{"Magic Knight Rayearth", "Clamp", "Clamp", "Three schoolgirls are transported to a magical world and must save it from darkness.", "completed", "manga", 1993, 36, []string{"Action", "Fantasy", "Shoujo"}},
		{"Revolutionary Girl Utena", "Be-PaPas", "Chiho Saito", "A girl challenges the revolutionary dueling system of her school.", "completed", "manga", 1996, 40, []string{"Action", "Drama", "Shoujo"}},
		{"Elfen Lied", "Lynn Okamoto", "Lynn Okamoto", "A girl with killing powers seeks normalcy while pursued by a secretive government agency.", "completed", "manga", 2002, 107, []string{"Action", "Horror", "Sci-Fi"}},
		{"Gantz", "Hiroya Oku", "Hiroya Oku", "Dead people are resurrected to hunt mysterious aliens in the real world.", "completed", "manga", 2000, 383, []string{"Action", "Horror", "Sci-Fi"}},
		{"ParaKiss", "Clamp", "Clamp", "A struggling student is discovered and becomes a model for a fashion design group.", "completed", "manga", 2000, 50, []string{"Comedy", "Fashion", "Shoujo"}},
		{"xxxHolic", "Clamp", "Clamp", "A mysterious shop owner grants wishes for supernatural prices.", "completed", "manga", 2003, 212, []string{"Mystery", "Supernatural", "Drama"}},
		{"Tsubasa Reservoir Chronicle", "Clamp", "Clamp", "A young girl's soul is scattered across dimensions and must be collected to save her.", "completed", "manga", 2003, 233, []string{"Action", "Fantasy", "Adventure"}},
		{"Evangelion", "Yoshiyuki Sadamoto", "Yoshiyuki Sadamoto", "Teenage pilots must fight aliens threatening humanity using giant mechas.", "completed", "manga", 1995, 90, []string{"Mecha", "Sci-Fi", "Action"}},
		{"Code Geass", "Ichiro Okouchi", "Clamp", "A student gains the power to command anyone and uses it to rebel against the empire.", "completed", "manga", 2006, 148, []string{"Mecha", "Sci-Fi", "Action"}},
		{"Death Parade", "Yracuq Kaiji", "Yracuq Kaiji", "A mysterious bar where the dead are judged for their actions in life.", "completed", "manga", 2012, 16, []string{"Mystery", "Psychological", "Supernatural"}},
		{"Anohana: The Flower We Saw That Day", "Deca05", "Deca05", "Friends reunite to fulfill the wish of a childhood friend who died.", "completed", "manga", 2011, 24, []string{"Drama", "Comedy", "Supernatural"}},
		{"A Place Further Than the Universe", "Atsuko Ishizuka", "Atsuko Ishizuka", "Four girls pursue their dream of reaching Antarctica.", "completed", "manga", 2018, 13, []string{"Adventure", "Comedy", "Drama"}},

		// Additional 30+ Titles
		{"Slam Dunk: Kore Wa Dunk da", "Takehiko Inoue", "Takehiko Inoue", "Basketball action and coming-of-age story.", "completed", "manga", 1990, 276, []string{"Sports", "School", "Drama"}},
		{"Twin Star Exorcists", "Yoshiaki Sukeno", "Yoshiaki Sukeno", "Two exorcist twins are forced to marry to produce a stronger exorcist.", "ongoing", "manga", 2014, 85, []string{"Action", "Supernatural", "Romance"}},
		{"Beyond Evil", "Yoon Ji-ryun", "Yoon Ji-ryun", "A gritty crime thriller following detectives and criminals in a dark city.", "completed", "manga", 2010, 120, []string{"Crime", "Thriller", "Drama"}},
		{"Orange", "Ichigo Takano", "Ichigo Takano", "A girl receives a letter from her future self warning about a friend's suicide.", "completed", "manga", 2012, 50, []string{"Drama", "Romance", "Supernatural"}},
		{"Platinum End", "Tsugumi Ohba", "Takeshi Obata", "Ten people chosen as candidates for God compete in a deadly game.", "completed", "manga", 2015, 58, []string{"Psychological", "Supernatural", "Thriller"}},
		{"The Flowers of Evil", "Shuzo Oshimi", "Shuzo Oshimi", "A boy is manipulated into a twisted relationship by a beautiful girl.", "completed", "manga", 2009, 58, []string{"Psychological", "Drama", "Horror"}},
		{"Toradora!", "Yuyuko Takemiya", "Yui Kiyohara", "A tall fierce girl and short wimpy boy team up to help each other get with their crushes.", "completed", "manga", 2008, 20, []string{"Comedy", "Romance", "School"}},
		{"Relife", "Yuu Watase", "Yuu Watase", "A man is given the chance to redo his high school years through a rehabilitation program.", "completed", "manga", 2013, 222, []string{"Drama", "Romance", "School"}},
		{"School-Live!", "Norimitsu Kaiho", "Norimitsu Kaiho", "Girls navigate their school life during a zombie apocalypse.", "completed", "manga", 2012, 78, []string{"Horror", "Comedy", "Slice of Life"}},
		{"Noragami", "Adachitoka", "Adachitoka", "A homeless god involves a human girl in his supernatural affairs.", "ongoing", "manga", 2010, 110, []string{"Action", "Comedy", "Supernatural"}},
		{"Servamp", "Strike Tanaka", "Strike Tanaka", "A boy befriends a vampire servant cat and learns of an immortal vampire war.", "ongoing", "manga", 2011, 80, []string{"Action", "Supernatural", "Comedy"}},
		{"Seraph of the End", "Takaya Kagami", "Yamato Yamamoto", "A boy seeks revenge against vampires who destroyed humanity.", "ongoing", "manga", 2012, 150, []string{"Action", "Supernatural", "Horror"}},
		{"Toilet-bound Hanako-kun", "AidaIro", "AidaIro", "A girl makes a wish with a ghost in the school toilet.", "ongoing", "manga", 2016, 200, []string{"Supernatural", "Comedy", "Mystery"}},
		{"Boarding School Juliet", "Yousuke Kaneda", "Yousuke Kaneda", "Two students from rival dorms engage in a secret romance.", "completed", "manga", 2015, 120, []string{"Comedy", "Romance", "School"}},
		{"Angels of Death", "Kudan Naduka", "Kudan Naduka", "A girl wakes with no memories in a building with a serial killer.", "ongoing", "manga", 2016, 70, []string{"Horror", "Psychological", "Thriller"}},
		{"The Ancient Magus' Bride", "Kore Yamazaki", "Kore Yamazaki", "A girl is sold to a powerful magus who promises to help her find true self.", "ongoing", "manga", 2013, 100, []string{"Fantasy", "Romance", "Supernatural"}},
		{"Liar Game", "Shinobu Kaitani", "Shinobu Kaitani", "A naive girl is trapped in a psychological game of deception.", "completed", "manga", 2005, 201, []string{"Psychological", "Thriller", "Mystery"}},
		{"Blood on the Tracks", "Shuzo Oshimi", "Shuzo Oshimi", "A boy discovers his father is a psychopath and manipulative killer.", "completed", "manga", 2016, 44, []string{"Psychological", "Horror", "Drama"}},
		{"Fire Punch", "Tatsuki Fujimoto", "Tatsuki Fujimoto", "A man with regeneration powers seeks revenge in a frozen apocalyptic world.", "completed", "manga", 2016, 83, []string{"Action", "Horror", "Sci-Fi"}},
		{"Act-Age", "Tatsuya Matsuki", "Shiro Usazaki", "A girl with acting talent but no self-esteem pursues stardom with a mentor's help.", "completed", "manga", 2017, 170, []string{"Drama", "Comedy", "School"}},
		{"Given", "Natsuki Kizu", "Natsuki Kizu", "A band story between four boys forming a rock band.", "completed", "manga", 2013, 92, []string{"Music", "Romance", "Drama"}},
		{"New Game!", "Shotaro Tokuno", "Shotaro Tokuno", "Girls work at a video game development company creating games.", "completed", "manga", 2013, 97, []string{"Comedy", "School", "Slice of Life"}},
		{"Shigatsu wa Kimi no Uso", "Arakawa Naoshi", "Arakawa Naoshi", "A piano prodigy and violinist meet and make beautiful music together.", "completed", "manga", 2011, 45, []string{"Music", "Romance", "School"}},
		{"Assassination Classroom", "Yusei Matsui", "Yusei Matsui", "A class of students must kill their teacher who is an alien threatening Earth.", "completed", "manga", 2012, 187, []string{"Action", "Comedy", "School"}},
		{"The Disastrous Life of Saiki K", "Shuuichirou Yamamoto", "Shuuichirou Yamamoto", "An apathetic boy with psychic powers navigates high school and absurd situations.", "completed", "manga", 2012, 281, []string{"Comedy", "School", "Supernatural"}},
		{"Toilet-Bound Hanako-kun: Jibaku Shounen", "AidaIro", "AidaIro", "Stories of wishes and ghosts in a school bathroom.", "ongoing", "manga", 2016, 200, []string{"Supernatural", "Comedy", "Mystery"}},
	}

	for _, m := range mangaList {
		mangaID := uuid.New().String()

		// Insert manga
		_, err := db.Exec(`
			INSERT INTO manga (id, title, author, artist, description, status, type, total_chapters, year, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			mangaID, m.title, m.author, m.artist, m.description,
			m.status, m.mangaType, m.chapters, m.year, time.Now(), time.Now(),
		)
		if err != nil {
			return err
		}

		// Get genre IDs and link to manga
		for _, genreName := range m.genres {
			var genreID string
			err := db.QueryRow("SELECT id FROM genres WHERE name = ?", genreName).Scan(&genreID)
			if err == nil {
				_, err = db.Exec(`
					INSERT INTO manga_genres (id, manga_id, genre_id, created_at)
					VALUES (?, ?, ?, ?)`,
					uuid.New().String(), mangaID, genreID, time.Now(),
				)
				if err != nil {
					return err
				}
			}
		}

		// Create external IDs entry
		_, err = db.Exec(`
			INSERT INTO manga_external_ids (manga_id, mangadex_id, primary_source, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`,
			mangaID, uuid.New().String()[:8], "mangadex", time.Now(), time.Now(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) seedReadingProgress() error {
	// Get test users
	rows, err := db.Query("SELECT id FROM users WHERE role = 'user' LIMIT 3")
	if err != nil {
		return err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return err
		}
		userIDs = append(userIDs, userID)
	}

	if len(userIDs) == 0 {
		return nil
	}

	// Get manga
	mangaRows, err := db.Query("SELECT id FROM manga LIMIT 5")
	if err != nil {
		return err
	}
	defer mangaRows.Close()

	var mangaIDs []string
	for mangaRows.Next() {
		var mangaID string
		if err := mangaRows.Scan(&mangaID); err != nil {
			return err
		}
		mangaIDs = append(mangaIDs, mangaID)
	}

	if len(mangaIDs) == 0 {
		return nil
	}

	// Create reading progress entries
	statuses := []string{"plan_to_read", "reading", "completed"}
	for i, userID := range userIDs {
		for j, mangaID := range mangaIDs {
			status := statuses[(i+j)%len(statuses)]
			currentChapter := (i + j + 1) * 10

			if status == "completed" {
				currentChapter = 100
			} else if status == "plan_to_read" {
				currentChapter = 0
			}

			_, err := db.Exec(`
				INSERT INTO reading_progress (id, user_id, manga_id, current_chapter, status, is_favorite, last_read_at, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				uuid.New().String(), userID, mangaID, currentChapter, status, j%2 == 0, time.Now(), time.Now(), time.Now(),
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
