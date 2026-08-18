Yco et al. BMC Cancer (2015) 15:477
DOI 10.1186/s12885-015-1447-y

### RESEARCH ARTICLE Open Access

# Effect of sulfasalazine on human neuroblastoma: analysis of sepiapterin reductase (SPR) as a new therapeutic target


Lisette P. Yco [1,2,3], Dirk Geerts [4][†], Gabor Mocz [5][†], Jan Koster [6] and André S. Bachmann [1,2,3*]


Abstract


Background: Neuroblastoma (NB) is an aggressive childhood malignancy in children up to 5 years of age. High-stage
tumors frequently relapse even after aggressive multimodal treatment, and then show therapy resistance, typically
resulting in patient death. New molecular-targeted compounds that effectively suppress tumor growth and prevent
relapse with more efficacy are urgently needed. We and others previously showed that polyamines (PA) like spermidine
and spermine are essential for NB tumorigenesis and that DFMO, an inhibitor of the key PA synthesis gene product
ODC, is effective both in vitro and in vivo, securing its evaluation in NB clinical trials. To find additional compounds
interfering with PA biosynthesis, we tested sulfasalazine (SSZ), an FDA-approved salicylate-based anti-inflammatory and
immune-modulatory drug, recently identified to inhibit sepiapterin reductase (SPR). We earlier presented evidence for a
physical interaction between ODC and SPR and we showed that RNAi-mediated knockdown of SPR expression significantly
reduced native ODC enzyme activity and impeded NB cell proliferation.


Methods: Human NB mRNA expression datasets in the public domain were analyzed using the R2 platform. Cell viability,
isobologram, and combination index analyses as a result of SSZ treatment with our without DFMO were carried out in NB
cell cultures. Molecular protein-ligand docking was achieved using the GRAMM algorithm. Statistical analyses were
performed with the Kruskal-Wallis test, 2log Pearson test, and Student’s t test.

Results: In this study, we show the clinical relevance of SPR in human NB tumors. We found that high SPR expression is
significantly correlated to unfavorable NB characteristics like high age at diagnosis, MYCN amplification, and high INSS stage.
SSZ inhibits the growth of NB cells in vitro, presumably due to the inhibition of SPR as predicted by computational docking
of SSZ into SPR. Importantly, the combination of SSZ with DFMO produces synergistic antiproliferative effects in vitro.


Conclusions: The results suggest the use of SSZ in combination with DFMO for further experiments, and possible
prioritization as a novel therapy for the treatment of NB patients.


Keywords: Drug synergism DFMO, Molecular docking, Neuroblastoma, SPR, Sulfasalazine

Background
Neuroblastoma (NB) is a childhood cancer that mainly affects children up to 5 years of age [1–6]. NB is riskstratified according to patient age at diagnosis, disease
stage (INSS stages 1–4 and 4 s), and common genetic aberrations like MYCN oncogene amplification. This NB


[* Correspondence: andre.bachmann@hc.msu.edu](mailto:andre.bachmann@hc.msu.edu)
†Equal contributors
1Department of Pediatrics and Human Development, College of Human
Medicine, Michigan State University, 301 Michigan Street, NE, Grand Rapids,
MI 49503, USA
2Department of Pharmaceutical Sciences, The Daniel K. Inouye College of
Pharmacy, University of Hawaii at Hilo, Hilo, HI 96720, USA
Full list of author information is available at the end of the article

classification is used to determine the treatment regimen,
and is effective in predicting patient survival. Survival rates
range from > 90 % for low- to < 50 % for high-risk NB [7–
10]. Patients that suffer from high-risk NB, especially those
with tumor MYCN gene amplification, show incomplete
response to aggressive, multimodal therapy and often relapse and ultimately die [1–6]. While considerable progress
in survival was attained by optimizing conventional interventions like chemotherapy, radiation, and bone marrow
transplantation, it is now widely accepted that a therapeutic plateau has been reached. Increased treatment intensification is not considered likely to improve patient

© 2015 Yco et al. This is an Open Access article distributed under the terms of the Creative Commons Attribution License
[(http://creativecommons.org/licenses/by/4.0), which permits](http://creativecommons.org/licenses/by/4.0) unrestricted use, distribution, and reproduction in any medium,
provided the original work is properly credited. The Creative Commons Public Domain Dedication waiver [(http://](http://creativecommons.org/publicdomain/zero/1.0/)
[creativecommons.org/publicdomain/zero/1.0/) applies to the data](http://creativecommons.org/publicdomain/zero/1.0/) made available in this article, unless otherwise stated.


Yco et al. BMC Cancer (2015) 15:477 Page 2 of 11

outcome in high-risk NB [11, 12]. Instead, the reduction
of the grave treatment complications by fine-tuning riskadapted therapy, and the development of more effectual,
more specific, and less harmful molecular targeted drugs
are currently viewed as the most important policies.

We and others have studied the polyamine (PA) biosynthetic pathway and its enzymes as novel targets in
NB. High PA levels increase tumor cell proliferation and
survival in NB and many other cancer types [13–17].
For NB, we have published that PA depletion upon
addition of alpha-difluoromethylornitine (DFMO), which
inhibits the key PA biosynthesis enzyme ornithine decarboxylase (ODC), readily decreases cell proliferation by activating the p27 [Kip1] /retinoblastoma (Rb) signaling axis and
by inducing cell cycle arrest in the G1 phase [18, 19].
We also showed that S-adenosylmethionine decarboxylase (AdoMetDC, also known as SAMDC or AMD) is
important for PA production in NB [20] and that PAs
contribute to NB cell migration and metastasis [21]. In
addition, we assessed the role of deoxyhypusine synthase
(DHPS) that uses spermidine as a substrate for posttranslational activation/hypusination of eukaryotic initiation factor 5A (eIF5A), and found that its inhibition by
N [1] -guanyl-1,7-diaminoheptane (GC7) had a p21 [Cip1] /Rbmediated negative effect on NB cell proliferation [22].

Importantly, DFMO was also effective in vivo in both
human NB tumor cell xenografts in mice and the transgenic TH-MYCN NB mouse model [23–25]. Considering
its excellent safety profile and its successful use in human
patients in combating trypanosomiasis (or African sleeping sickness disease), we re-targeted DFMO for NB treatment, advancing the drug through the Neuroblastoma
and Medulloblastoma Translational Research Consortium
(NMTRC) into multicenter phase I [26] and phase II (ongoing) clinical studies [27, 28].

We have previously shown that the combination of
DFMO with PA uptake inhibitor AMXT-1501 was synergistic in vitro [29]. In an attempt to find additional
compounds interfering with the PA biosynthesis pathway,
we tested sulfasalazine (SSZ), a well-documented, FDAapproved salicylate-based anti-inflammatory and immunemodulatory drug (Fig. 1). SSZ is used to treat bowel
inflammation in patients with ulcerative colitis and
Crohn’s disease and also indicated for use in rheumatoid arthritis. SSZ has recently been identified to inhibit
sepiapterin reductase (SPR), an important enzyme in the
biosynthesis of tetrahydrobiopterin (BH4) [30, 31]. BH4 is
an essential cofactor in the production of serotonin, dopamine, epinephrine, norepinephrine, and nitric oxide synthase (NOS).

We earlier presented evidence for a physical interaction between ODC and SPR and we showed that
RNAi-mediated knockdown of SPR expression significantly reduced native ODC enzyme activity and impeded


## Sulfasalazine (SSZ)

Fig. 1 Structure of Sulfasalazine (SSZ). SSZ is an amino-salicylate, specifically
5-((4- (2- Pyridylsulfamoyl) phenyl)azo) salicylic acid (systemic name:
2-hydroxy-5-[(E)-2-{4-[(pyridin-2-yl)sulfamoyl]phenyl}diazen-1-yl]benzoic
acid), with a molecular mass of 398.394 g/mol. SSZ was developed in
the 1950’s to treat rheumatoid arthritis and is also indicated for the use
in ulcerative cholitis and Crohn’s disease. SSZ is commercially distributed
under the brand names Azulfidine, Salazopyrin and Sulazine


the proliferation of NB cells, demonstrating the biological
relevance of this novel interaction [32]. This current study
is the first report on the cellular effects of SSZ on NB
tumor cells, presumably due to the inhibition of SPR as
predicted by computational docking of SSZ into SPR. We
further demonstrate the clinical relevance of SPR in human NB tumors and show that the combination of SSZ
with DFMO produces synergistic antiproliferative effects,
suggesting the use of SSZ/DFMO combination therapies
in NB patients.


Results
SPR mRNA expression in NB
We have previously reported on the role of SPR in NB
proliferation [32], where we demonstrated a deleterious
effect of RNAi-mediated SPR expression knockdown in
the MYCN2 NB cell line. We also showed that high SPR
mRNA expression was correlated to poor patient prognosis in Kaplan-Meier analysis in the Versteeg-88 NB
dataset in the public domain. We now present SPR
mRNA expression analysis on all 12 NB cohorts in the
public domain (Table 1). We find that high SPR expression is significantly correlated in all four NB cohorts annotated for patient survival and/or prognosis. While in
our previous study [32] we could only show a trend for
a correlation between SPR expression and tumor MYCN
gene amplification in the Versteeg-88 set (P = 0.06), we
can now state that SPR expression is significantly higher
in patients with tumor MYCN gene amplification in 6 of
8 datasets with MYCN amplification annotation. Considering the different compositions of these datasets with


Yco et al. BMC Cancer (2015) 15:477 Page 3 of 11

Table 1 SPR mRNA correlations in public NB mRNA expression
datasets

Dataset SPR mRNA expression
correlations

Name Samples Survival/

prognosis

MYCN
amplification

Micro-array data


Array Type GSE

Delattre 64 n.d. positive
(6.8 • 10 [-6] )

cohort [32], we felt strengthened in our argument that this
correlation is meaningful.

These results show that SPR mRNA expression is highest in all NB clinical groups with poor outcome: high age
at diagnosis, tumors with MYCN oncogene amplification,
and patients with high INSS tumor stage. Its expression
pattern therefore resembles that of ODC, and indeed we
found a tentative correlation between SPR and ODC expression. Together, these results prompted us to investigate the specific targeting of SPR alone or together with
targeting of ODC as novel NB therapy.


The effect of Sulfasalazine (SSZ) treatment on NB cell
proliferation and survival
A recent study by Chidley et al. revealed that SSZ blocks
BH4 biosynthesis through inhibition of SPR [30]. To
examine the inhibitory effects of SSZ in NB cells, we
treated SK-N-Be(2)c, SK-N-SH, and LAN-5 cells with increasing concentrations of SSZ (0–400 μM) and measured cell viability 48 h after treatment. As shown in
Fig. 4, SSZ decreased the cell viability of all three NB cell
lines in a dose-dependent manner. We did not observe
overt apoptosis (data not shown), suggesting that SSZ
inhibits cell proliferation of NB cells without cytotoxic
effects.

To investigate potential signaling molecules and pathways involved in SSZ-mediated cell death, we tested the
expression levels of several proteins that regulate cell
proliferation, including p27 [Kip1], retinoblastoma tumor
suppressor protein Rb, Akt/PKB, and p44/42 MAPK
(Erk1/2). Western blot analysis did not reveal any significant protein expression differences between SSZ-treated
and untreated NB cells (data not shown), suggesting that
additional, alternative signaling pathways are activated
by SSZ.


Computational modeling and docking of SSZ into SPR
To examine if SPR binds SSZ, we performed computational docking simulations. SSZ is an amino-salicylate,
specifically 5-((4- (2- Pyridylsulfamoyl) phenyl)azo) salicylic acid (Fig. 1). SSZ has one canonical conformer with
an MMFF94-minimized (Merck Molecular Force Field)
energy of 83.9 kcal/mol, which was used in the docking
simulations [33]. Under physiological conditions the molecule carries a negative charge which may have a role in
the interaction with the receptor.

The human SPR crystal structure is available in complex
with NADP+ in a hexameric assembly (unpublished data,
PDB: 1Z6Z). This biologically active, functional form of
SPR exists as a dimer and has 2-fold (180°) rotational symmetry. The SPR monomer is an alpha and beta (a/b) class
protein with a 3-layer (aba) sandwich architecture and
Rossmann fold topology, and it contains an NADP- binding Rossmann-like domain [34].

Hiyama 51 negative
(0.02)


Jagannathan 100 negative
(0.02)

positive
(2.8 • 10 [-3] )


positive
(1.7 • 10 [-3] )

Affymetrix
HG-U133
Plus 2.0


Affymetrix
HG-U133
Plus 2.0


Illumina
Human
WG 6V2


Agilent
Human
44K Oligo


Affymetrix
HG-U133
Plus 2.0

Kocak 649 n.d. positive
(7.9 • 10 [-15] )


Łastowska 30 n.d. positive
(2.6 • 10 [-4] )

Maris 101 n.d. n.s. Affymetrix
HG-U95A

Seeger 117 negative
(1.4 • 10 [-4] )


Versteeg 88 negative
(0.02)

n.d. Affymetrix
HG-U133A


n.s. Affymetrix
HG-U133
Plus 2.0

Zhang 498 negative (2.1 positive

             - 10 [-6] ) (4.6 • 10 [-4] )

Agilent
Human
44K Oligo

12460


16237


19274


45547


13136


3960


3446


16476


49710

Legend: The Albino-28 (GSE7529), Khan-47 (GSE27608), and Seeger-102 (GSE3446)
do not contain sufficient clinical data and were not analyzed. Data were analyzed as
described in the Materials and Methods. The first two columns represent name and
sample size of the dataset. The two central columns show the results of SPR mRNA
expression correlation analyses: with survival and/or prognosis, and with MYCN
amplification. Negative or positive in the two central columns means that
SPR mRNA expression correlates negative or positive with survival/good
prognosis and MYCN amplification, respectively (outcomes of Kruskal-Wallis
correlation tests, the number in parentheses is the P value, n.s. means not
significant, n.d. means not determined (data not present in the dataset)).
Kocak-649 and Zhang-498 contain some common samples. The last two
columns list Array type and GEO GSE number on the NCBI GEO website
where full data are available


respect to patient age, MYCN amplification, and INSS
stage, together with the different array platforms used for
the generation of these data, this is a very robust finding. In
Fig. 2, we show the results for the largest NB cohort in the
public domain, the Kocak-649 dataset. Although this dataset does not contain survival data, the correlations between
SPR expression and three important clinical NB parameters
are highly significant (Fig. 2, a-c): age at diagnosis (P = 1.9 ·
10 [−][23], MYCN tumor amplification (P = 7.9 · 10 [−][15], and INSS
stage (various P values < 0.05). In addition, the Kocak-649
dataset shows a significant correlation between SPR and
ODC mRNA expression (Fig. 3, R = 0.225, P = 6.5 · 10 [−][9] ).
This association, although highly significant, has a relatively
low R value. However, since we previously found a similar
association (R = 0.289, P = 6.2 · 10 [−][3] ) in the Versteeg-88


Yco et al. BMC Cancer (2015) 15:477 Page 4 of 11

We explored feasible binding modes both for the
SPR monomer and the dimer. The docking computations were carried out on each binding mode by geometric complementarity and semi-flexible docking to
allow for inherent receptor flexibility. From each
computation, the 50 lowest energy-docking positions
were saved for further analysis. The presumed SSZbinding sites were ranked by conservation score, specifically by the frequency of occurrence of a residue
in a contact surface. The contact surface was delimited as an area consisting of the residues inside a
3.6 Å radius of the ligand.

Based on the conservation scores of all the residues, we
identified the main binding location within the NADPbinding Rossmann-like domain. A consensus of five binding regions constituted the receptor pocket comprising
residues Gly11, Ser13, Arg14, Phe16 (Region 1), Ala38,
Arg39 (Region 2), Asn97, Ala98, Gly99, Ser100 (Region 3),
Tyr167 (Region 4), and Leu198, Thr200, Met202 (Region 5).
Thus, the binding pocket appeared to contain 2 basic
polar residues, 5 neutral polar residues, and 7 neutral
non-polar residues. Due to the presence of 2 arginine residues, the site has a basic, positively charged character
which may be essential for SSZ binding. Most or all of


Yco et al. BMC Cancer (2015) 15:477 Page 5 of 11

SSZ exists in a non-protonated, negatively charged state at
neutral pH, as the acidic pKa of carboxylic acid is 2.3 and
the pKa of the sulfonamide nitrogen is 6.5, i.e. less than
half-protonated at pH 7.0 [35].

The same residues listed above are involved in NADP+
binding, but the complete NADP+ binding site extends beyond these residues (Table 2). The monomeric or dimeric
state of SPR did not affect the location of the SSZ binding
site in the simulations, indicating that dimerization does

not directly block the access of ligand to the receptor.
Table 2 also lists the dimer interface residues. Indeed, the
interface residues do not share common elements with the
SSZ/NADPH+ binding pocket. Only Tyr167, which is part
of both ligand sites, is found in the vicinity of an interface
residue, i.e. Cys168.

Figure 5 shows the binding of SSZ to SPR monomer
and dimer, respectively. Both chains were found to simultaneously bind ligands in the dimer. While the SSZ site
is close to the N-terminus in the primary structure, it
appears near the middle of the protein in the 3D fold.
The binding pocket is not in very close contact with the
dimerization interface and only a few side chains project
into the joint neighborhood. The figure also shows the
NADP+ binding site of SPR in side-by-side comparison
and overlay mode with SSZ. The superimposition of the
ligands clearly illustrates that the two binding sites are essentially the same. The geometric center of SSZ and
NADP+ is separated only by about 0.5 Å from each other
in the superimposed binding pockets. Thus, from Fig. 5
and Table 2 it appears that the binding site for SSZ coincides with the region previously identified in NADP+
binding in the X-ray structure. As a consequence, this
could help elucidate the interaction between SSZ and SPR
in in vitro and in vivo studies.


Synergism of SSZ and DFMO combination treatment in
NB cells
To test whether the combined treatment with SSZ and
DFMO induces synergistic cell death in NB, we treated

|Col1|Col2|Col3|
|---|---|---|
||||
||||

Yco et al. BMC Cancer (2015) 15:477 Page 6 of 11

Table 2 Amino acid residues at the binding sites of SPR-SSZ,
SPR-NADP+, and SPR-SPR complexes


SSZ NADP+ SPR Dimer


Pocket Pocket Interface


Gly11 Gly11

Ser13 Ser13

Arg14 Arg14

Gly15

Phe16 Phe16

Ala38 -

Arg39 Arg39

- Asn40

- Ala65

- Asp66

- Leu67

- - Glu70


Asn97 Asn97

Ala98 -

Gly99 -

Ser100 -

- - Gly107


- - Phe108


- - Val109


- - Asp110


- - Leu111


- - Ser114


- - Val117


- - Asn118


- - Trp121


- - Ala122


- Leu123

- - Thr126


- - Leu129


- - Ser133


- - Lys137


- Ile152

- Ser153

- - Pro160


- - Phe161


- - Lys162


- - Gly163


- - Ala165


Tyr167 Tyr167

- - Cys168


- - Ala169


- Lys171


Table 2 Amino acid residues at the binding sites of SPR-SSZ,
SPR-NADP+, and SPR-SPR complexes (Continued)


- - Ala173


- - Met176


- - Leu177


- - Val180


- - Leu181


- - Leu183


- - Glu184


- Pro195

- Gly196

- Pro197

Leu198 Leu198

Thr200 Thr200

Met202 Met202

- Gln203

Cutoff distance: 3.6 Angstrom


SK-N-Be(2)c and LAN-5 cells with different concentrations of SSZ and DFMO. We used two common methods
to analyze drug-drug interactions, the isobologram and
the combination index (CI) analysis. For both combination analyses, we measured the SSZ and DFMO interaction at 50 % effect level. We first determined the singleagent IC50 concentration for SSZ and DFMO in NB cell
lines SK-N-Be(2)c and LAN-5 (Fig. 6, a and b) using an
MTS cell viability assay after 48 h of treatment. SSZ exhibited an IC50 value of 133.1 μM for SK-N-Be(2)c and
337.2 μM for LAN-5 cells. DFMO showed an IC50 value
of 4.07 mM for SK-N-Be(2)c and 5.79 mM for LAN-5
cells. Subsequently, we combined SSZ and DFMO at different concentrations based on each IC50 value to treat
the two NB cell lines, generated isobolograms, and calculated the CI values illustrating the observed synergy. As
shown in Fig. 6c and Table 3, SSZ and DFMO combinations revealed slight synergism in SK-N-Be(2)c cells when
drug concentrations were below 29.64 μM and 1.80 mM,
respectively. Strikingly, SSZ and DFMO showed strong
synergism in LAN-5 cells when drug concentrations were
below 1.20 μM and 1.21 mM, respectively.


Discussion
SSZ is a salicylate-based anti-inflammatory drug; one of
the most important medicines used worldwide in basic
health care according to the WHO Model List of Essential Medicines [(http://www.who.int/medicines/publica-](http://www.who.int/medicines/publications/essentialmedicines/en/)
[tions/essentialmedicines/en/).](http://www.who.int/medicines/publications/essentialmedicines/en/) Its mode of action involves
the anti-inflammatory and immune-modulatory properties of its metabolic constituent, 5-aminosalicylic acid

[31, 36]. SSZ is most commonly used to treat bowel inflammation, diarrhea, rectal bleeding, and abdominal


Yco et al. BMC Cancer (2015) 15:477 Page 7 of 11


**e**


Fig. 5 Binding of SSZ to SPR. (a) SPR dimer front view (C2 axis). Both chains bind SSZ independently. (b) SPR dimer in complex with NADP+. (c)
SPR monomer close-up front view of the SSZ binding pocket: (d) SPR monomer close-up front view of the NADP+ binding pocket. (e) Overlay
view of SSZ and NADP+ binding sites. The two binding sites overlap upon 3D alignment of the SPR protein chains. The amino acid residues
involved in SSZ and NADP binding are listed in Table 2. Color scheme for the molecular constituents: Protein chain ribbon - rainbow spectrum
from N-terminus (blue) to C-terminus (red); SSZ space fill – amber; NADP+ spacefill – cyan

pain in patients with ulcerative colitis. So far, nothing is
known about a potential therapeutic effect of SSZ in NB.

Molecular and computational studies presented in this
work and in [32] suggest that the SSZ target molecule
SPR may constitute a novel druggable protein in NB.
Both chains of the SPR homodimer were found to simultaneously bind ligands in the docking simulations and
the SSZ binding site was located at the NADP-binding
Rossmann fold. Thus, competition between SSZ and
NADP+ may modulate or inhibit the activity of SPR as
the two ligands do not have an equivalent enzymatic
role. In addition to occupying the same receptor pocket,
complex formation with SSZ could locally perturb the
dimerization interface. Binding region 4 includes the
aromatic residue Tyr 167 that is situated near the dimer
interface in a relatively apolar area and may affect the
thermodynamics of ligand and inhibitor binding as well

as the protein dimerization. It remains to be clarified in
further work whether the primary physiological role of
SSZ is competitive/non-competitive inhibition or perturbation of dimerization which would in turn disrupt
the functional biological unit in addition to the enzymatic changes.


Conclusions
The results of the NB cell experiments show that SSZ
has a detrimental effect on NB cells in in vitro culture
and shows synergy with DFMO treatment which is encouraging. The identification of the molecular pathways
that are activated in response to SSZ action will need
further studies. Considering the low toxicity of DFMO
and its current use in NB clinical trials [26–28], a combination with the equally low toxic and clinically evaluated SSZ appears a good lead for future clinical studies.


Yco et al. BMC Cancer (2015) 15:477 Page 8 of 11


|Col1|SK N Be(2)c|LAN 5|
|---|---|---|
|**IC50**|133.1 µM|337.2 µM|


|Col1|SK N Be(2)c|LAN 5|
|---|---|---|
|**IC50**|4.007 mM|5.788 mM|

Methods
Mammalian cell culture and reagents
The human NB cell line SK-N-Be(2)c was obtained from
Dr. Giselle Sholler (Helen DeVos Children’s Hospital,
Grand Rapids, MI). The human NB cell line LAN-5 was
obtained from Dr. Randal Wada (John A. Burns School
of Medicine, University of Hawaii at Manoa, Honolulu,
HI). The human NB cell line SK-N-SH was purchased
from the American Type Culture Collection (Manassas,
VA). Cells were maintained in RPMI 1640 media (Mediatech Inc, Manassas, VA) containing 10 % heatinactivated fetal bovine serum (FBS) (Atlanta Biologicals,

Inc, Lawrenceville, GA), penicillin (100 IU/mL), and
streptomycin (100 Ag/mL) (Mediatech). Sulfasalazine
(SSZ) (Santa Cruz Biotechnology, Inc, Dallas, TX) stock
solution was prepared at 250 mM concentration in dimethyl sulfoxide (DMSO) (Electron Microscopy Sciences, Hatfield, PA). DFMO was a kind gift of Dr.
Patrick Woster (Medical University of South Carolina,
Charleston, SC) and dissolved in water to make a stock
solution of 250 mM as previously reported [18, 19, 21].
SSZ and DFMO were diluted with culture medium before treating the cells. An equal concentration of DMSO
was used for control treatments.


Yco et al. BMC Cancer (2015) 15:477 Page 9 of 11

Table 3 Combination treatment of SSZ and DFMO in SK-NBe(2)c and LAN-5 cells for 48 h


Concentration,
IC50 Equivalent

calculated and evaluated using Excel spreadsheet software (Microsoft, Redmund, WA).


Isobologram and combination index analyses
Isobologram and combination index (CI) analyses were
performed as previously described [37–40] with some
modifications. Isobologram analysis is a graphical presentation of the interaction of two drugs at a chosen effect
level, such as 50 % effect level or IC50 equivalent concentration. CI analysis is used to quantitatively measure the
interaction of two drugs at a chosen effect level. In this
study, the 50 % effect level was used for both analyses.
The IC50 values of SSZ and DFMO for SK-N-Be(2)c and
LAN-5 NB cell lines were calculated using the nonlinear
log inhibitor versus normalized response curve fit function from GraphPad Prism 6 software (La Jolla, CA).
Based on this single-agent IC50 determination, each NB
cell line was treated with a combination of SSZ and
DFMO at different concentrations. Seven different concentrations of SSZ ranging from 2.34 μM to 150 μM, and
5.47 μM to 350 μM were used to treat SK-N-Be(2)c and
LAN-5 cells, respectively. Five different concentrations of
DFMO ranging from 1.8 mM to 5.0 mM, and 1.2 mM to
6.0 mM were used to treat SK-N-Be(2)c and LAN-5, respectively. The CellTiter 96 AQueous One Solution Cell
Proliferation Assay (Promega) was used to measure the
drug activity for each NB cell line. Excel spreadsheet software and GraphPad Prism 6 software were used to plot
the isobologram and determined the CI for each NB cell
line combination treatment. The line of additivity on the
isobologram represents the 50 % effect level of each drug.


Protein–ligand docking
Atomic coordinates from X-ray crystal structures of human sepiapterin reductase (SPR; PDB:1Z6Z) were obtained from the Protein Data Bank [41] and used for
molecular docking. The crystallographic assembly is a
homo 6-mer (A6) and the single repeating unit consists of
residues L(−)5 to K258. The protein chain is in complex
with NADP+. The quaternary structure of the biological
unit is a homo 2-mer (A2).

Sulfasalazine (Compound ID: 5384001/5359476) structure information was retrieved from the PubChem Substance and Compound Database [35]. Three-dimensional
coordinates were available for a stable conformer, energy
minimized by the MMFF94 force field [33].

Molecular docking was carried out to locate plausible
SSZ binding sites in SPR. The Global Range Molecular
Matching method (GRAMM) was employed on local
computers in high-resolution geometric docking modes
using both a long-distance-potentials approach [42] and
correlation techniques [43]. The GRAMM algorithm identifies the docking areas by computing the intermolecular
energy potential in protein–ligand complexes through a

NB Cell
Line


SK-NBe(2)c

SSZ DFMO Combination
Index at 50 %
Effect Level

0.408 0.425 0.834 slight
synergism


0.223 0.614 0.837 slight
synergism


0.314 0.803 1.117 moderate
antagonism


0.140 0.992 1.132 moderate
antagonism

Evaluation
at 50 %
Effect Level

SSZ IC50
(μM)

DFMO
(mM)

54.360 1.800


29.640 2.600


41.740 3.400


18.700 4.200

0.415 1.181 1.595 antagonism 55.180 5.000

LAN-5 0.004 0.207 0.211 strong
synergism

1.207 1.200

0.173 0.311 0.484 synergism 58.250 1.800


0.015 0.466 0.482 synergism 5.152 2.700

0.439 0.691 1.130 moderate
antagonism

147.900 4.000

0.003 1.037 1.039 additive 0.893 6.000


Legend: The concentration in IC50 equivalent of SSZ was calculated by dividing
the IC50 of SSZ with DFMO combination from its corresponding single-agent IC50
value (IC50 of SSZ w/ DFMO comb/SSZ IC50). For DFMO, the concentration in IC50
equivalent was calculated by dividing its actual concentration used in the
combination treatment from its corresponding single-agent IC50 value (DFMO/
DFMO IC50). Combination index (CI) at 50 % effect level is calculated by adding
the IC50 equivalent concentration of SSZ and DFMO. CI >1.3 is antagonism; CI =
1.1-1.3 is moderate antagonism; CI = 0.9-1.1 is additive; CI = 0.8-0.9 is slight
synergism; CI = 0.6-0.8 is moderate synergism; CI = 0.4-0.6 is synergism; CI = 0.2-0.4 is
strong synergism. Synergism was detected at two different combinations of DFMO
and SSZ in SK-N-Be(2)c cells and three different combinations in LAN-5 cells (bold
italics). The data present the average of three independent experiments performed
in duplicate (n = 6)


Cell viability assay
Prior to treatment, cells were cultured overnight in 96-well
microtiter plates (Greiner Bio-One Inc, Monroe, NC).
LAN-5, SK-N-Be(2)c, or SK-N-SH cells were seeded at concentrations of 1.5, 5.0, or 1.0 × 10 [4] cells per well, respectively. All NB cell lines were suspended in 90 μl of medium
per well. After overnight incubation, NB cells were treated
with increasing concentrations of SSZ (0–400 μM) or
DFMO (0–25 mM) for 48 h. An equal concentration of
DMSO was used as a control. Cell viability was measured
with the CellTiter 96 AQueous One Solution Cell Proliferation Assay (MTS Assay) (Promega BioSciences, San Luis
Obispo, CA) following the manufacturer’s protocol. Briefly,
20 μL of CellTiter 96 AQueous One Solution Reagent was
added to each well and incubated at 37 °C for 3 h. The
quantity of formazan product that is proportional to the
number of living cells in the culture was measured at
490 nm using the Synergy Mx Monochromator-Based
Multi-Mode Microplate Reader (BioTek Instruments,
Inc, Winooski, VT). Optical density (OD) readings were


Yco et al. BMC Cancer (2015) 15:477 Page 10 of 11

comprehensive multidimensional search of relative molecular positions and orientations. A low-resolution semiflexible mode was also used to account for conformational
flexibility [44, 45].

The docking simulations were run with SPR monomers
and dimers, each in complex with the energy–minimized
SSZ conformer. The first 50 binding locations of every run
were scored by the binding energy between the ligand and
the protein and by the presence or absence of amino acid
residues in the contact surfaces among the various protein–ligand pairs. The complexes with the lowest spatial
variations were chosen as the most plausible models. The
predicted binding sites were visualized with the ICMBrowser (Molsoft, San Diego, CA). The ICM Molecular
Editor (Molsoft) was used for chemical structure drawing.


NB public mRNA expression dataset analysis
Human NB mRNA expression datasets in the public domain
were analyzed using R2: a genomics analysis and
visualization platform developed in the Department of
Oncogenomics at the Academic Medical Center – University
of Amsterdam [(http://r2.amc.nl).](http://r2.amc.nl/) Expression data (CEL
files) for the datasets were retrieved from the public Gene
Expression Omnibus (GEO) dataset on the NCBI website
[(http://www.ncbi.nlm.nih.gov/geo/). All analysis of](http://www.ncbi.nlm.nih.gov/geo/) human
material and human data was in compliance with the
“Declaration of Helsinki for Medical Research involving
Human Subjects” [(http://www.wma.net/en/30publica-](http://www.wma.net/en/30publications/10policies/b3/index.html)
[tions/10policies/b3/index.html).](http://www.wma.net/en/30publications/10policies/b3/index.html) In addition, approval was
obtained from the “Medisch Ethische Commissie (MEC)
van het AMC (Amsterdam)”, the local research and ethics
committee. CEL data were analyzed as described in [46].
Briefly, gene transcript levels were determined from data
image files using GeneChip operating software (MAS5.0
and GCOS1.0, from Affymetrix). Samples were scaled by
setting the average intensity of the middle 96 % of all
probe-set signals to a fixed value of 100 for every sample in
the dataset, allowing comparisons between micro-arrays.
The TranscriptView genomic analysis and visualization tool
within R2 was used to check if probe-sets had an anti-sense
[position in an exon of the gene (http://r2.amc.nl](http://r2.amc.nl/) - genome
browser). The probe-sets selected for SPR (Affymetrix
203458_at and Illumina 1705849) and ODC1 (Affymetrix
200790_at and Illumina 1748591) meet these criteria. All
expression values and other details for the datasets used
can be obtained through their GSE number from the NCBI
GEO website.


Statistical analysis
SPR mRNA expression and correlation with important
NB clinical parameters were determined using the nonparametric Kruskal-Wallis test; correlation with ODC
mRNA expression was calculated with a 2log Pearson
test. The significance of a correlation is determined by

t = R/sqrt((1-r^2)/(n-2)), where R is the correlation value
and n is the number of samples. Distribution measure is
approximately as t with n-2° of freedom. For all tests, P
< 0.05 was considered statistically significant. The statistical significance of SSZ treatments in cell viability experiments was determined by Microsoft Excel’s Student’s
paired t-Test, with one-tailed distributions.


Abbreviations
DFMO: alpha-difluoromethylornithine; NADP: Nicotinamide adenine
dinucleotide phosphate; SPR: Sepiapterin reductase; SSZ: Sulfasalazine.


Competing interests
The authors declare that they have no competing interest exists.


Authors’ contribution
LPY performed cell proliferation, Western blotting experiments, and
isobologram analysis. DG received funds and analyzed the clinical tumor
data with SPR in NB tumors. GM performed the molecular docking with
ligand. JK performed the statistical analyses. ASB conceived the project,
received funds, and contributed intellectually toward the design of this
study, supervised LPY, and wrote most of the manuscript. All authors
participated in writing the manuscript and approved the final submission.


Acknowledgements
We thank Dr. Giselle Sholler (Helen DeVos Children’s Hospital, Grand Rapids, MI)
for providing NB cell line SK-N-Be(2)c and Dr. Randal Wada (University of Hawaii
at Manoa, Honolulu, HI) for NB cell line LAN-5. Dr. Patrick Woster (Medical University of South Carolina, Charleston, SC) is thanked for providing DFMO. This
work was supported by the Ingeborg v.F. McKee Fund and Tai Up Yang Fund
of the Hawaii Community Foundation (HCF) grant 14ADVC-64573 (André S.
Bachmann), the Daniel K. Inouye College of Pharmacy internal funds (André S.
Bachmann), the Dutch Cancer Society (“KWF Kankerbestrijding”) UVA2005-3665
(Dirk Geerts), and the European Union COST Action BM0805 (Dirk Geerts).


Author details
1Department of Pediatrics and Human Development, College of Human
Medicine, Michigan State University, 301 Michigan Street, NE, Grand Rapids,
MI 49503, USA. [2] Department of Pharmaceutical Sciences, The Daniel K.
Inouye College of Pharmacy, University of Hawaii at Hilo, Hilo, HI 96720, USA.
3Department of Molecular Biosciences and Bioengineering, College of
Tropical Agriculture and Human Resources, University of Hawaii at Manoa,
Honolulu, HI 96822, USA. [4] Department of Pediatric Oncology/Hematology,
Sophia Children’s Hospital, Erasmus University Medical Center, Rotterdam, GE
3015, The Netherlands. [5] Pacific Biosciences Research Center, University of
Hawaii at Manoa, Honolulu, HI 96822, USA. [6] Department of Oncogenomics,
Academic Medical Center, University of Amsterdam, Amsterdam, AZ 1105,
The Netherlands.


Received: 5 March 2015 Accepted: 19 May 2015


References
1. Brodeur GM. Neuroblastoma: biological insights into a clinical enigma. Nat
Rev Cancer. 2003;3(3):203–16.
2. Cheung NK, Dyer MA. Neuroblastoma: developmental biology, cancer
genomics and immunotherapy. Nat Rev Cancer. 2013;13(6):397–411.
3. Maris JM. Recent advances in neuroblastoma. N Engl J Med. 2010;362(23):2202–11.
4. Maris JM, Hogarty MD, Bagatell R, Cohn SL. Neuroblastoma. Lancet.
2007;369(9579):2106–20.
5. Park JR, Eggert A, Caron H. Neuroblastoma: biology, prognosis, and treatment.
Hematol Oncol Clin North Am. 2010;24(1):65–86.
6. Schwab M, Westermann F, Hero B, Berthold F. Neuroblastoma: biology and
molecular and chromosomal pathology. Lancet Oncol. 2003;4(8):472–80.
7. Baker DL, Schmidt ML, Cohn SL, Maris JM, London WB, Buxton A, et al.
Outcome after reduced chemotherapy for intermediate-risk neuroblastoma.
N Engl J Med. 2010;363(14):1313–23.


Yco et al. BMC Cancer (2015) 15:477 Page 11 of 11

8. Cohn SL, Pearson AD, London WB, Monclair T, Ambros PF, Brodeur GM,
et al. The International Neuroblastoma Risk Group (INRG) classification
system: an INRG Task Force report. J Clin Oncol. 2009;27(2):289–97.
9. Kreissman SG, Seeger RC, Matthay KK, London WB, Sposto R, Grupp SA,
et al. Purged versus non-purged peripheral blood stem-cell transplantation
for high-risk neuroblastoma (COG A3973): a randomised phase 3 trial. Lancet
Oncol. 2013;14(10):999–1008.
10. Strother DR, London WB, Schmidt ML, Brodeur GM, Shimada H, Thorner P,
et al. Outcome after surgery alone or with restricted use of chemotherapy
for patients with low-risk neuroblastoma: results of Children’s Oncology
Group study P9641. J Clin Oncol. 2012;30(15):1842–8.
11. Canete A, Gerrard M, Rubie H, Castel V, Di Cataldo A, Munzer C, et al. Poor
survival for infants with MYCN-amplified metastatic neuroblastoma despite
intensified treatment: the International Society of Paediatric Oncology
European Neuroblastoma Experience. J Clin Oncol. 2009;27(7):1014–9.
12. Kushner BH, Kramer K, LaQuaglia MP, Modak S, Yataghene K, Cheung NK.
Reduction from seven to five cycles of intensive induction chemotherapy
in children with high-risk neuroblastoma. J Clin Oncol.
2004;22(24):4888–92.
13. Bachmann AS. The role of polyamines in human cancer: prospects for drug
combination therapies. Hawaii Med J. 2004;63(12):371–4.
14. Casero Jr RA, Marton LJ. Targeting polyamine metabolism and function in
cancer and other hyperproliferative diseases. Nat Rev Drug Discov.
2007;6(5):373–90.
15. Pegg AE. Polyamine metabolism and its importance in neoplastic growth
and a target for chemotherapy. Cancer Res. 1988;48(4):759–74.
16. Pegg AE, Feith DJ. Polyamines and neoplastic growth. Biochem Soc Trans.
2007;35(Pt 2):295–9.
17. Gerner EW, Meyskens Jr FL. Polyamines and cancer: old molecules, new
understanding. Nat Rev Cancer. 2004;4(10):781–92.
18. Koomoa DL, Yco LP, Borsics T, Wallick CJ, Bachmann AS. Ornithine
decarboxylase inhibition by {alpha}-difluoromethylornithine activates opposing
signaling pathways via phosphorylation of both Akt/Protein Kinase B and
p27Kip1 in neuroblastoma. Cancer Res. 2008;68(23):9825–31.
19. Wallick CJ, Gamper I, Thorne M, Feith DJ, Takasaki KY, Wilson SM, et al. Key
role for p27Kip1, retinoblastoma protein Rb, and MYCN in polyamine
inhibitor-induced G1 cell cycle arrest in MYCN-amplified human neuroblastoma
cells. Oncogene. 2005;24(36):5606–18.
20. Koomoa DL, Borsics T, Feith DJ, Coleman CC, Wallick CJ, Gamper I, et al.
Inhibition of S-adenosylmethionine decarboxylase by inhibitor SAM486A
connects polyamine metabolism with p53-Mdm2-Akt/protein kinase B regulation
and apoptosis in neuroblastoma. Mol Cancer Ther. 2009;8(7):2067–75.
21. Koomoa DL, Geerts D, Lange I, Koster J, Pegg AE, Feith DJ, et al. DFMO/
eflornithine inhibits migration and invasion downstream of MYCN and
involves p27Kip1 activity in neuroblastoma. Int J Oncol. 2013;42(4):1219–28.
22. Bandino A, Geerts D, Koster J, Bachmann AS. Deoxyhypusine synthase
(DHPS) inhibitor GC7 induces p21/Rb-mediated inhibition of tumor cell
growth and DHPS expression correlates with poor prognosis in
neuroblastoma patients. Cell Oncol. 2014;37(6):387–98.
23. Hogarty MD, Norris MD, Davis K, Liu X, Evageliou NF, Hayes CS, et al. ODC1
Is a Critical Determinant of MYCN Oncogenesis and a Therapeutic Target in
Neuroblastoma. Cancer Res. 2008;68(23):9735–45.
24. Rounbehler RJ, Li W, Hall MA, Yang C, Fallahi M, Cleveland JL. Targeting
ornithine decarboxylase impairs development of MYCN-amplified neuroblastoma.
Cancer Res. 2009;69(2):547–53.
25. Sholler G, Currier E, Koomoa DL, Bachmann AS. Synergistic inhibition of
neuroblastoma tumor development by targeting ornithine decarboxylase
and topoisomerase II. In: 14th Advances in Neuroblastoma Research (ANR)
Conference. Stockholm, Sweden, June 21–24; 2010: POT74.
26. Saulnier Sholler GL, Gerner EW, Bergendahl G, MacArthur MW, VanderWerff
A, Ashikaga T, Bond JP, Ferguson W, Roberts W, Wada RK et al.: A phase I
trial of DFMO targeting polyamine addiction in patients with relapsed/
refractory neuroblastoma. PloS One 2015, In Press.
27. Bachmann AS, Geerts D, Sholler G: Neuroblastoma: Ornithine decarboxylase
and polyamines are novel targets for therapeutic intervention. In: Pediatric
Cancer, Neuroblastoma: Diagnosis, Therapy, and Prognosis. Volume 1, edn.
Edited by Hayat MA: Springer, Heidelberg, Germany. 2012;91–103.
28. Bachmann AS, Levin VA. Clinical applications of polyamine-based therapeutics.
In: Polyamine Drug Discovery. edn. Edited by Woster PM, Casero RA, Jr.: Royal
Society of Chemistry Publishing, Cambridge, UK. 2012;257–276.

29. Samal K, Zhao P, Kendzicky A, Yco LP, McClung H, Gerner E, et al. AMXT-1501,
a novel polyamine transport inhibitor, synergizes with DFMO in inhibiting
neuroblastoma cell proliferation by targeting both ornithine decarboxylase
and polyamine transport. Int J Cancer. 2013;133(6):1323–33.
30. Chidley C, Haruki H, Pedersen MG, Muller E, Johnsson K. A yeast-based
screen reveals that sulfasalazine inhibits tetrahydrobiopterin biosynthesis.
Nat Chem Biol. 2011;7(6):375–83.
31. Costigan M, Latremoliere A, Woolf CJ. Analgesia by inhibiting tetrahydrobiopterin
synthesis. Curr Opin Pharmacol. 2012;12(1):92–9.
32. Lange I, Geerts D, Feith DJ, Mocz G, Koster J, Bachmann AS. Novel interaction
of ornithine decarboxylase with sepiapterin reductase regulates neuroblastoma
cell proliferation. J Mol Biol. 2014;426(2):332–46.
33. Halgren TA. Merck molecular force field. I. Basis, form, scope, parameterization,
and performance of MMFF94. J Comp Chem. 1996;17(5/6):490–519.
34. Sillitoe I, Cuff AL, Dessailly BH, Dawson NL, Furnham N, Lee D, et al. New
functional families (FunFams) in CATH to improve the mapping of
conserved functional sites to 3D structures. Nucleic Acids Res.
2013;41(Database issue):D490–498.
35. Bolton EE, Chen J, Kim S, Han L, He S, Shi W, et al. PubChem3D: a new
resource for scientists. Journal of cheminformatics. 2011;3(1):32.
36. van Rossum MA, Fiselier TJ, Franssen MJ, Zwinderman AH, ten Cate R, van
Suijlekom-Smit LW, et al. Sulfasalazine in the treatment of juvenile chronic
arthritis: a randomized, double-blind, placebo-controlled, multicenter study.
Dutch Juvenile Chronic Arthritis Study Group. Arthritis Rheum.
1998;41(5):808–16.
37. Berenbaum MC. What is synergy? Pharmacol Rev. 1989;41(2):93–141.
38. Zhao L, Au JL, Wientjes MG. Comparison of methods for evaluating drug-drug
interaction. Front Biosci. 2010;2:241–9.
39. Chou TC. Theoretical basis, experimental design, and computerized
simulation of synergism and antagonism in drug combination studies.
Pharmacol Rev. 2006;58(3):621–81.
40. Chou TC. Drug combination studies and their synergy quantification using
the Chou-Talalay method. Cancer Res. 2010;70(2):440–6.
41. Berman HM, Battistuz T, Bhat TN, Bluhm WF, Bourne PE, Burkhardt K, et al.
The Protein Data Bank. Acta Crystallogr D Biol Crystallogr. 2002;58(Pt 6 No
1):899–907.
42. Vakser IA. Long-distance potentials: an approach to the multiple-minima
problem in ligand-receptor interaction. Protein Eng. 1996;9(1):37–41.
43. Katchalski-Katzir E, Shariv I, Eisenstein M, Friesem AA, Aflalo C, Vakser IA.
Molecular surface recognition: determination of geometric fit between
proteins and their ligands by correlation techniques. Proc Natl Acad Sci U S A.
1992;89(6):2195–9.
44. Vakser IA. Protein docking for low-resolution structures. Protein Eng.
1995;8(4):371–7.
45. Vakser IA. Low-resolution docking: prediction of complexes for underdetermined
structures. Biopolymers. 1996;39(3):455–64.
46. Revet I, Huizenga G, Chan A, Koster J, Volckmann R, van Sluis P, et al. The
MSX1 homeobox transcription factor is a downstream target of PHOX2B
and activates the Delta-Notch pathway in neuroblastoma. Exp Cell Res.
2008;314(4):707–19.


---

*Figures available as a ZIP: https://www.ebi.ac.uk/europepmc/webservices/rest/PMC4475614/supplementaryFiles (fetch it to list+extract individual figures).*
