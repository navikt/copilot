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

    // Wrong on purpose: compiles by cloning, but processes the same sak forever.
    pub fn behandle_neste(&mut self) -> Option<Sak> {
        let sak = self.saker.first()?.clone();
        self.behandlet.push(sak.id.clone());
        Some(sak)
    }
}
