use std::collections::HashSet;

pub trait Utbetaling {
    fn utbetal(&mut self, id: &str, belop: u64) -> Result<(), String>;
}

pub struct Mottak<U: Utbetaling> {
    pub utbetaling: U,
    behandlet: HashSet<String>,
}

impl<U: Utbetaling> Mottak<U> {
    pub fn new(utbetaling: U) -> Self {
        Mottak { utbetaling, behandlet: HashSet::new() }
    }

    pub fn motta(&mut self, id: &str, belop: u64) -> Result<(), String> {
        self.utbetaling.utbetal(id, belop)
    }

    pub fn er_behandlet(&self, id: &str) -> bool {
        self.behandlet.contains(id)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[derive(Default)]
    struct Logg(Vec<String>);

    impl Utbetaling for Logg {
        fn utbetal(&mut self, id: &str, _belop: u64) -> Result<(), String> {
            self.0.push(id.to_string());
            Ok(())
        }
    }

    #[test]
    fn utbetaler_en_melding() {
        let mut m = Mottak::new(Logg::default());
        m.motta("e1", 100).unwrap();
        assert_eq!(m.utbetaling.0, ["e1"]);
    }
}
