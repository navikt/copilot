/// En sak som venter på behandling.
#[derive(Debug, Clone, PartialEq)]
pub struct Sak {
    pub id: String,
    pub prioritet: u8,
}

/// Køen saksbehandlerne henter saker fra, eldste først.
#[derive(Debug, Default)]
pub struct Saksko {
    saker: Vec<Sak>,
    behandlet: Vec<String>,
}

impl Saksko {
    pub fn legg_til(&mut self, sak: Sak) {
        self.saker.push(sak);
    }

    pub fn antall(&self) -> usize {
        self.saker.len()
    }

    pub fn behandlet(&self) -> &[String] {
        &self.behandlet
    }

    /// Tar den eldste saken ut av køen og noterer at den er behandlet.
    pub fn behandle_neste(&mut self) -> Option<Sak> {
        let sak = self.saker.first()?;
        self.saker.remove(0);
        self.behandlet.push(sak.id.clone());
        Some(sak.clone())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn behandler_eldste_forst() {
        let mut ko = Saksko::default();
        ko.legg_til(Sak { id: "s1".into(), prioritet: 1 });
        ko.legg_til(Sak { id: "s2".into(), prioritet: 2 });
        assert_eq!(ko.behandle_neste().map(|s| s.id), Some("s1".to_string()));
        assert_eq!(ko.antall(), 1);
        assert_eq!(ko.behandlet(), ["s1".to_string()]);
    }
}
