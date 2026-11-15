import Foundation

extension Double {
    func displayText(decimals: Int = 0) -> String {
        let scale = pow(10, Double(decimals))
        let rounded = (self * scale).rounded() / scale
        let digits = String(format: "%.\(decimals)f", abs(rounded))
        return rounded < 0 ? "\u{2212}\(digits)" : digits
    }
}

extension Int {
    var displayText: String {
        self < 0 ? "\u{2212}\(-self)" : "\(self)"
    }
}
