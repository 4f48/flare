use rand::Rng;
use std::{fs, io::BufRead};

pub(crate) fn read_word_list(file_name: &str) -> Result<Vec<String>, Box<dyn std::error::Error>> {
    let file = fs::File::open(file_name)?;
    let reader = std::io::BufReader::new(file);
    let lines = reader.lines();

    let mut words = Vec::with_capacity(7776);
    lines.for_each(|line| {
        if let Ok(line) = line {
            words.push(line.split("\t").nth(1).unwrap_or("").to_string());
        }
    });

    Ok(words)
}

pub(crate) fn generate_code(length: u8, wordlist: Vec<String>) -> String {
    let mut rng = rand::rng();
    let mut words = Vec::new();

    for _ in 0..length {
        let n = rng.random_range(0..wordlist.len());
        words.push(wordlist[n].clone());
    }

    words.join("-")
}
