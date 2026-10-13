/// Et vedtak lest fra en linje i filen fra fagsystemet.
#[derive(Debug, PartialEq)]
pub struct Vedtak {
    pub id: String,
    pub belop: u32,
}

/// Leser en linje på formen "id;belop".
pub fn les(linje: &str) -> Vedtak {
    let mut deler = linje.split(';');
    let id = deler.next().unwrap().to_string();
    let belop = deler.next().unwrap().parse().unwrap();
    Vedtak { id, belop }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn leser_gyldig_linje() {
        assert_eq!(les("v1;250"), Vedtak { id: "v1".into(), belop: 250 });
    }
}
